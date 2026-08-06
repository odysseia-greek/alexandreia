package scholar

import (
	"context"
	"fmt"
	"time"

	queuepb "github.com/odysseia-greek/agora/eupalinos/v1"
	"github.com/odysseia-greek/agora/plato/config"
	"github.com/odysseia-greek/agora/plato/logging"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
	"google.golang.org/protobuf/proto"
)

const (
	EnvQueuePollInterval     = "ARISTARCHOS_QUEUE_POLL_INTERVAL"
	DefaultQueueName         = "aristarchos"
	DefaultQueuePollInterval = time.Second
)

func (a *AggregatorServiceImpl) StartQueueListener(ctx context.Context, pollInterval time.Duration) {
	if pollInterval <= 0 {
		pollInterval = DefaultQueuePollInterval
	}

	logging.Info(fmt.Sprintf("listening for queued aristarchos words on channel: %s", a.QueueName))

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Info("stopped aristarchos queue listener")
			return
		default:
		}

		err := a.ProcessNextQueueMessage(ctx)
		if err == nil {
			continue
		}

		if ctx.Err() != nil {
			logging.Info("stopped aristarchos queue listener")
			return
		}

		logging.Debug(fmt.Sprintf("queue dequeue returned: %s", err.Error()))

		select {
		case <-ctx.Done():
			logging.Info("stopped aristarchos queue listener")
			return
		case <-ticker.C:
		}
	}
}

func (a *AggregatorServiceImpl) ProcessNextQueueMessage(ctx context.Context) error {
	if a.Queue == nil {
		return fmt.Errorf("queue client is not configured")
	}

	queueName := a.QueueName
	if queueName == "" {
		queueName = DefaultQueueName
	}

	message, err := a.Queue.DequeueMessageBytes(ctx, &queuepb.ChannelInfo{Name: queueName, AckMode: true})
	if err != nil {
		return err
	}

	request := &v1.AggregatorCreationRequest{}
	if err := proto.Unmarshal(message.Data, request); err != nil {
		return a.nackQueueMessage(ctx, queueName, message.Id, fmt.Errorf("unmarshal queued aggregator creation request: %w", err))
	}

	processCtx := traceContext(ctx, request.TraceId)
	if err := a.createOrUpdate(processCtx, request); err != nil {
		return a.nackQueueMessage(ctx, queueName, message.Id, err)
	}

	acknowledged, err := a.Queue.AcknowledgeMessage(ctx, &queuepb.AcknowledgeRequest{Channel: queueName, Id: message.Id})
	if err != nil {
		return fmt.Errorf("acknowledge queue message %s: %w", message.Id, err)
	}
	if !acknowledged.GetAcknowledged() {
		return fmt.Errorf("queue message %s was not acknowledged", message.Id)
	}
	return nil
}

func (a *AggregatorServiceImpl) nackQueueMessage(ctx context.Context, channel, id string, processingErr error) error {
	nacked, err := a.Queue.NackMessage(ctx, &queuepb.NackRequest{Channel: channel, Id: id})
	if err != nil {
		return fmt.Errorf("%w; nack queue message %s: %v", processingErr, id, err)
	}
	if !nacked.GetRequeued() {
		return fmt.Errorf("%w; queue message %s was not requeued", processingErr, id)
	}
	return processingErr
}

func QueuePollIntervalFromEnv(value string) time.Duration {
	if value == "" {
		return DefaultQueuePollInterval
	}

	interval, err := time.ParseDuration(value)
	if err != nil {
		logging.Warn(fmt.Sprintf("invalid aristarchos queue poll interval %q, using default %s", value, DefaultQueuePollInterval))
		return DefaultQueuePollInterval
	}

	return interval
}

func traceContext(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}

	traceID, spanID, traceCall := comedy.TraceFromString(requestID)
	if traceID == "" || spanID == "" || !traceCall {
		return ctx
	}

	return context.WithValue(ctx, config.DefaultTracingName, requestID)
}
