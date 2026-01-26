package scholar

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/aristoteles"
	"github.com/odysseia-greek/agora/plato/randomizer"
	"github.com/odysseia-greek/agora/plato/service"
	v1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type ExactService interface {
	WaitForHealthyState() bool
	Search(ctx context.Context, request *koinos.SearchQuery) (*v1.SearchResponse, error)
}

const (
	DEFAULTADDRESS string = "localhost:50060"
)

type ExactServiceImpl struct {
	Elastic    aristoteles.Client
	Index      string
	Version    string
	Randomizer randomizer.Random
	Client     service.OdysseiaClient
	Streamer   arv1.TraceService_ChorusClient
	Archytas   archytas.Client
	v1.UnimplementedHefastionServiceServer

	totalRequests atomic.Uint64
	ipMap         sync.Map
}

type ExactServiceClient struct {
	Impl ExactService
}
type ExactClient struct {
	exact v1.HefastionServiceClient
}

func NewHefaistionClient(address string) (*ExactClient, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tracing service: %w", err)
	}
	client := v1.NewHefastionServiceClient(conn)
	return &ExactClient{exact: client}, nil
}

func (e *ExactClient) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := e.Health(context.Background(), &emptypb.Empty{})
		if err == nil && response.Healthy {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (e *ExactClient) Health(ctx context.Context, request *emptypb.Empty) (*koinos.HealthResponse, error) {
	return e.exact.Health(ctx, request)
}

func (e *ExactClient) Search(ctx context.Context, request *koinos.SearchQuery) (*v1.SearchResponse, error) {
	return e.exact.Search(ctx, request)
}
