package grammar

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/odysseia-greek/alexandreia/dionysios/gen/go/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ScholarService interface {
	WaitForHealthyState() bool
	CheckGrammar(ctx context.Context, request *v1.CheckGrammarRequest) (*v1.CheckGrammarResponse, error)
	Research(ctx context.Context, request *v1.ResearchRequest) (*v1.ResearchResponse, error)
}

const (
	DEFAULTADDRESS = "localhost:50060"
)

type ScholarClient struct {
	grammar v1.DionysiosServiceClient
}

func NewDionysiosClient(address string) (*ScholarClient, error) {
	if address == "" {
		address = DEFAULTADDRESS
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dionysios service: %w", err)
	}

	client := v1.NewDionysiosServiceClient(conn)
	return &ScholarClient{grammar: client}, nil
}

func (s *ScholarClient) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := s.Health(context.Background(), &v1.HealthRequest{})
		if err == nil && response.Healthy {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

func (s *ScholarClient) Ping(ctx context.Context, request *v1.PingRequest) (*v1.PingResponse, error) {
	return s.grammar.Ping(ctx, request)
}

func (s *ScholarClient) Health(ctx context.Context, request *v1.HealthRequest) (*v1.HealthResponse, error) {
	return s.grammar.Health(ctx, request)
}

func (s *ScholarClient) CheckGrammar(ctx context.Context, request *v1.CheckGrammarRequest) (*v1.CheckGrammarResponse, error) {
	return s.grammar.CheckGrammar(ctx, request)
}

func (s *ScholarClient) Research(ctx context.Context, request *v1.ResearchRequest) (*v1.ResearchResponse, error) {
	return s.grammar.Research(ctx, request)
}
