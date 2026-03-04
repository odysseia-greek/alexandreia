package scholia

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseia-greek/agora/aristoteles"
	aristarchos "github.com/odysseia-greek/alexandreia/aristarchos/scholar"
	v1 "github.com/odysseia-greek/alexandreia/kallimachos/gen/go/v1"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type LibraryService interface {
	WaitForHealthyState() bool
	Analyze(ctx context.Context, entry *v1.AnalyzeRequest) (*v1.AnalyzeResponse, error)
}

const (
	DEFAULTADDRESS = "localhost:50060"
)

type LibraryServiceImpl struct {
	Elastic    aristoteles.Client
	Index      string
	Version    string
	Streamer   arv1.TraceService_ChorusClient
	Aggregator *aristarchos.ClientAggregator
	v1.UnimplementedKallimachosServiceServer
}

type LibraryClient struct {
	library v1.KallimachosServiceClient
}

func NewKallimachosClient(address string) (*LibraryClient, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to kallimachos service: %w", err)
	}

	client := v1.NewKallimachosServiceClient(conn)
	return &LibraryClient{library: client}, nil
}

func (l *LibraryClient) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := l.Health(context.Background(), &emptypb.Empty{})
		if err == nil && response.Healthy {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (l *LibraryClient) Health(ctx context.Context, request *emptypb.Empty) (*v1.HealthResponse, error) {
	return l.library.Health(ctx, request)
}

func (l *LibraryClient) Analyze(ctx context.Context, request *v1.AnalyzeRequest) (*v1.AnalyzeResponse, error) {
	return l.library.Analyze(ctx, request)
}
