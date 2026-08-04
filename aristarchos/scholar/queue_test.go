package scholar

import (
	"context"
	"fmt"
	"testing"
	"time"

	queuepb "github.com/odysseia-greek/agora/eupalinos/v1"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

type fakeQueueService struct {
	message *queuepb.EpistelloBytes
	err     error
	channel string
	ackMode bool
	acked   string
	nacked  string
}

func (f *fakeQueueService) WaitForHealthyState() bool {
	return true
}

func (f *fakeQueueService) EnqueueMessageBytes(ctx context.Context, in *queuepb.EpistelloBytes) (*queuepb.EnqueueResponse, error) {
	return &queuepb.EnqueueResponse{Id: "queued"}, nil
}

func (f *fakeQueueService) DequeueMessageBytes(ctx context.Context, in *queuepb.ChannelInfo) (*queuepb.EpistelloBytes, error) {
	f.channel = in.Name
	f.ackMode = in.AckMode
	if f.err != nil {
		return nil, f.err
	}

	return f.message, nil
}

func (f *fakeQueueService) AcknowledgeMessage(ctx context.Context, in *queuepb.AcknowledgeRequest) (*queuepb.AcknowledgeResponse, error) {
	f.acked = in.Id
	return &queuepb.AcknowledgeResponse{Acknowledged: true}, nil
}

func (f *fakeQueueService) NackMessage(ctx context.Context, in *queuepb.NackRequest) (*queuepb.NackResponse, error) {
	f.nacked = in.Id
	return &queuepb.NackResponse{Requeued: true, NackCount: 1}, nil
}

func TestProcessNextQueueMessageCreatesNewWord(t *testing.T) {
	request := &v1.AggregatorCreationRequest{
		RootWord:     "λέγω",
		Word:         "λέγω",
		Rule:         "1st sing - pres - ind - act",
		Translation:  "I say",
		PartOfSpeech: v1.PartOfSpeech_VERB,
	}
	data, err := proto.Marshal(request)
	assert.Nil(t, err)

	queue := &fakeQueueService{
		message: &queuepb.EpistelloBytes{
			Id:      "message-1",
			Channel: DefaultQueueName,
			Data:    data,
		},
	}
	service := newTestAggregatorService(t, emptyHit, createDocument)
	service.Queue = queue
	service.QueueName = DefaultQueueName

	err = service.ProcessNextQueueMessage(context.Background())

	assert.Nil(t, err)
	assert.Equal(t, DefaultQueueName, queue.channel)
	assert.True(t, queue.ackMode)
	assert.Equal(t, "message-1", queue.acked)
	assert.Empty(t, queue.nacked)
}

func TestProcessNextQueueMessageRejectsMalformedPayload(t *testing.T) {
	queue := &fakeQueueService{
		message: &queuepb.EpistelloBytes{
			Id:      "message-2",
			Channel: DefaultQueueName,
			Data:    []byte("not a protobuf"),
		},
	}
	service := newTestAggregatorService(t, emptyHit, createDocument)
	service.Queue = queue
	service.QueueName = DefaultQueueName

	err := service.ProcessNextQueueMessage(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal queued aggregator creation request")
	assert.Equal(t, "message-2", queue.nacked)
	assert.Empty(t, queue.acked)
}

func TestProcessNextQueueMessageReturnsQueueError(t *testing.T) {
	queue := &fakeQueueService{err: fmt.Errorf("message queue is empty")}
	service := AggregatorServiceImpl{
		Queue:     queue,
		QueueName: DefaultQueueName,
	}

	err := service.ProcessNextQueueMessage(context.Background())

	assert.EqualError(t, err, "message queue is empty")
	assert.Equal(t, DefaultQueueName, queue.channel)
}

func TestQueuePollIntervalFromEnv(t *testing.T) {
	assert.Equal(t, 250*time.Millisecond, QueuePollIntervalFromEnv("250ms"))
	assert.Equal(t, DefaultQueuePollInterval, QueuePollIntervalFromEnv(""))
	assert.Equal(t, DefaultQueuePollInterval, QueuePollIntervalFromEnv("not-a-duration"))
}
