package scholia

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseia-greek/agora/aristoteles"
	ariv1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	v1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ScholarService interface {
	WaitForHealthyState() bool
	Analyze(ctx context.Context, entry *v1.AnalyzeRequest) (*v1.AnalyzeResponse, error)
}

type AggregatorResolver interface {
	RetrieveEntry(ctx context.Context, request *ariv1.AggregatorRequest) (*ariv1.RootWordResponse, error)
}

const (
	DEFAULTADDRESS = "localhost:50060"
)

type ScholarServiceImpl struct {
	Elastic    aristoteles.Client
	Index      string
	Version    string
	Streamer   arv1.TraceService_ChorusClient
	Aggregator AggregatorResolver
	v1.UnimplementedKallimachosServiceServer
}

type ScholarClient struct {
	library v1.KallimachosServiceClient
}

func NewKallimachosClient(address string) (*ScholarClient, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to kallimachos service: %w", err)
	}

	client := v1.NewKallimachosServiceClient(conn)
	return &ScholarClient{library: client}, nil
}

func (s *ScholarClient) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := s.Health(context.Background(), &emptypb.Empty{})
		if err == nil && response.Healthy {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (s *ScholarClient) Health(ctx context.Context, request *emptypb.Empty) (*v1.HealthResponse, error) {
	return s.library.Health(ctx, request)
}

func (s *ScholarClient) Analyze(ctx context.Context, request *v1.AnalyzeRequest) (*v1.AnalyzeResponse, error) {
	return s.library.Analyze(ctx, request)
}
