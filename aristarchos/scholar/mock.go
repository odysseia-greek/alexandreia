package scholar

import (
	"context"
	"time"

	pb "github.com/odysseia-greek/olympia/aristarchos/proto"
	"github.com/stretchr/testify/mock"
)

// MockAggregatorService is a mock implementation of the AggregatorService interface
type MockAggregatorService struct {
	mock.Mock
}

// WaitForHealthyState checks the health of the service within a timeout period
func (m *MockAggregatorService) WaitForHealthyState() bool {
	timeout := 30 * time.Second
	checkInterval := 1 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		response, err := m.Health(context.Background(), &pb.HealthRequest{})
		if err == nil && response.Health {
			return true
		}

		time.Sleep(checkInterval)
	}

	return false
}

// Health is a mock implementation of the Health method
func (m *MockAggregatorService) Health(ctx context.Context, request *pb.HealthRequest) (*pb.HealthResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*pb.HealthResponse), args.Error(1)
}

// CreateNewEntry is a mock implementation of the CreateNewEntry method
func (m *MockAggregatorService) CreateNewEntry(ctx context.Context) (pb.Aristarchos_CreateNewEntryClient, error) {
	args := m.Called(ctx)
	// Ensure that the type assertion matches the interface type
	return args.Get(0).(pb.Aristarchos_CreateNewEntryClient), args.Error(1)
}

// RetrieveEntry is a mock implementation of the RetrieveEntry method
func (m *MockAggregatorService) RetrieveEntry(ctx context.Context, request *pb.AggregatorRequest) (*pb.RootWordResponse, error) {
	args := m.Called(ctx, request)
	return args.Get(0).(*pb.RootWordResponse), args.Error(1)
}

// RetrieveRootFromGrammarForm is a mock implementation of the RetrieveRootFromGrammarForm method
func (m *MockAggregatorService) RetrieveRootFromGrammarForm(ctx context.Context, in *pb.AggregatorRequest) (*pb.FormsResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.FormsResponse), args.Error(1)
}

// RetrieveSearchWords is a mock implementation of the RetrieveSearchWords method
func (m *MockAggregatorService) RetrieveSearchWords(ctx context.Context, in *pb.AggregatorRequest) (*pb.SearchWordResponse, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*pb.SearchWordResponse), args.Error(1)
}
