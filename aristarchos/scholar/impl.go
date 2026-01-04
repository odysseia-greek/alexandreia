package scholar

import (
	"context"
	"fmt"
	"github.com/odysseia-greek/agora/aristoteles"
	pb "github.com/odysseia-greek/olympia/aristarchos/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type AggregatorService interface {
	WaitForHealthyState() bool
	CreateNewEntry(ctx context.Context) (pb.Aristarchos_CreateNewEntryClient, error)
	RetrieveEntry(ctx context.Context, request *pb.AggregatorRequest) (*pb.RootWordResponse, error)
	RetrieveRootFromGrammarForm(ctx context.Context, in *pb.AggregatorRequest) (*pb.FormsResponse, error)
	RetrieveSearchWords(ctx context.Context, in *pb.AggregatorRequest) (*pb.SearchWordResponse, error)
}

const (
	DEFAULTADDRESS string = "localhost:50060"
)

type AggregatorServiceImpl struct {
	Elastic    aristoteles.Client
	Index      string
	PolicyName string
	pb.UnimplementedAristarchosServer
}

type AggregatorServiceClient struct {
	Impl AggregatorService
}

type ClientAggregator struct {
	scholar pb.AristarchosClient
}

func NewClientAggregator(address string) (*ClientAggregator, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tracing service: %w", err)
	}
	client := pb.NewAristarchosClient(conn)
	return &ClientAggregator{scholar: client}, nil
}

func (c *ClientAggregator) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := c.Health(context.Background(), &pb.HealthRequest{})
		if err == nil && response.Health {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (c *ClientAggregator) Health(ctx context.Context, request *pb.HealthRequest) (*pb.HealthResponse, error) {
	return c.scholar.Health(ctx, request)
}

func (c *ClientAggregator) CreateNewEntry(ctx context.Context) (pb.Aristarchos_CreateNewEntryClient, error) {
	return c.scholar.CreateNewEntry(ctx)
}

func (c *ClientAggregator) RetrieveEntry(ctx context.Context, request *pb.AggregatorRequest) (*pb.RootWordResponse, error) {
	return c.scholar.RetrieveEntry(ctx, request)
}

func (c *ClientAggregator) RetrieveSearchWords(ctx context.Context, request *pb.AggregatorRequest) (*pb.SearchWordResponse, error) {
	return c.scholar.RetrieveSearchWords(ctx, request)
}

func (c *ClientAggregator) RetrieveRootFromGrammarForm(ctx context.Context, request *pb.AggregatorRequest) (*pb.FormsResponse, error) {
	return c.scholar.RetrieveRootFromGrammarForm(ctx, request)
}
