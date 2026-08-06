package grammar

import (
	"context"
	"testing"

	queuepb "github.com/odysseia-greek/agora/eupalinos/v1"
	"github.com/odysseia-greek/agora/plato/models"
	pba "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

type fakeAggregatorQueue struct {
	message *queuepb.EpistelloBytes
}

func (f *fakeAggregatorQueue) WaitForHealthyState() bool {
	return true
}

func (f *fakeAggregatorQueue) EnqueueMessageBytes(ctx context.Context, in *queuepb.EpistelloBytes) (*queuepb.EnqueueResponse, error) {
	f.message = in
	return &queuepb.EnqueueResponse{Id: "queued"}, nil
}

func (f *fakeAggregatorQueue) DequeueMessageBytes(ctx context.Context, in *queuepb.ChannelInfo) (*queuepb.EpistelloBytes, error) {
	return nil, nil
}

func TestSendWordsToAggregatorQueuesProtobufMessage(t *testing.T) {
	queue := &fakeAggregatorQueue{}
	handler := DionysosHandler{
		AggregatorQueue:   queue,
		AggregatorChannel: defaultAggregatorChannel,
	}
	declensions := &models.DeclensionTranslationResults{
		Results: []models.Result{
			{
				Word:        "λόγον",
				Rule:        "noun - sing - masc - acc",
				RootWord:    "λόγος",
				Translation: []string{"word"},
			},
		},
	}

	err := handler.sendWordsToAggregator(t.Context(), declensions, "trace+span+1")

	assert.Nil(t, err)
	assert.Equal(t, defaultAggregatorChannel, queue.message.Channel)

	request := &pba.AggregatorCreationRequest{}
	err = proto.Unmarshal(queue.message.Data, request)
	assert.Nil(t, err)
	assert.Equal(t, "λόγον", request.Word)
	assert.Equal(t, "λόγος", request.RootWord)
	assert.Equal(t, "word", request.Translation)
	assert.Equal(t, pba.PartOfSpeech_NOUN, request.PartOfSpeech)
	assert.Equal(t, "trace+span+1", request.TraceId)
}
