package scholar

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseia-greek/agora/aristoteles"
	queuepb "github.com/odysseia-greek/agora/eupalinos/proto"
	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AggregatorService interface {
	WaitForHealthyState() bool
	CreateNewEntry(ctx context.Context) (v1.Aristarchos_CreateNewEntryClient, error)
	RetrieveEntry(ctx context.Context, request *v1.AggregatorRequest) (*v1.RootWordResponse, error)
	RetrieveRootFromGrammarForm(ctx context.Context, in *v1.AggregatorRequest) (*v1.FormsResponse, error)
	RetrieveSearchWords(ctx context.Context, in *v1.AggregatorRequest) (*v1.SearchWordResponse, error)
}

type QueueService interface {
	WaitForHealthyState() bool
	EnqueueMessageBytes(ctx context.Context, in *queuepb.EpistelloBytes) (*queuepb.EnqueueResponse, error)
	DequeueMessageBytes(ctx context.Context, in *queuepb.ChannelInfo) (*queuepb.EpistelloBytes, error)
}

const (
	DEFAULTADDRESS string = "localhost:50060"
)

type AggregatorServiceImpl struct {
	Elastic    aristoteles.Client
	Queue      QueueService
	QueueName  string
	Index      string
	PolicyName string
	Streamer   arv1.TraceService_ChorusClient
	v1.UnimplementedAristarchosServer
}

type AggregatorServiceClient struct {
	Impl AggregatorService
}

type ClientAggregator struct {
	scholar v1.AristarchosClient
}

func NewClientAggregator(address string) (*ClientAggregator, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tracing service: %w", err)
	}
	client := v1.NewAristarchosClient(conn)
	return &ClientAggregator{scholar: client}, nil
}

func (c *ClientAggregator) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := c.Health(context.Background(), &v1.HealthRequest{})
		if err == nil && response.Health {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (c *ClientAggregator) Health(ctx context.Context, request *v1.HealthRequest) (*v1.HealthResponse, error) {
	return c.scholar.Health(ctx, request)
}

func (c *ClientAggregator) CreateNewEntry(ctx context.Context) (v1.Aristarchos_CreateNewEntryClient, error) {
	return c.scholar.CreateNewEntry(ctx)
}

func (c *ClientAggregator) RetrieveEntry(ctx context.Context, request *v1.AggregatorRequest) (*v1.RootWordResponse, error) {
	return c.scholar.RetrieveEntry(ctx, request)
}

func (c *ClientAggregator) RetrieveSearchWords(ctx context.Context, request *v1.AggregatorRequest) (*v1.SearchWordResponse, error) {
	return c.scholar.RetrieveSearchWords(ctx, request)
}

func (c *ClientAggregator) RetrieveRootFromGrammarForm(ctx context.Context, request *v1.AggregatorRequest) (*v1.FormsResponse, error) {
	return c.scholar.RetrieveRootFromGrammarForm(ctx, request)
}
