package library

import (
	"context"
	"fmt"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/aristoteles"
	v1 "github.com/odysseia-greek/alexandreia/eratosthenes/gen/go/v1"
	arv1 "github.com/odysseia-greek/attike/aristophanes/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type LibraryService interface {
	WaitForHealthyState() bool
}

const (
	DEFAULTADDRESS string = "localhost:50060"
)

type LibraryServiceImpl struct {
	Elastic  aristoteles.Client
	Index    string
	Version  string
	Streamer arv1.TraceService_ChorusClient
	Archytas archytas.Client
	v1.UnimplementedEratosthenesServiceServer
}

type LibraryServiceClient struct {
	Impl LibraryService
}
type LibraryClient struct {
	library v1.EratosthenesServiceClient
}

func NewEratosthenesClient(address string) (*LibraryClient, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tracing service: %w", err)
	}
	client := v1.NewEratosthenesServiceClient(conn)
	return &LibraryClient{library: client}, nil
}

func (d *LibraryClient) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := d.Health(context.Background(), &emptypb.Empty{})
		if err == nil && response.Healthy {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (d *LibraryClient) Health(ctx context.Context, request *emptypb.Empty) (*v1.HealthResponse, error) {
	return d.library.Health(ctx, request)
}
