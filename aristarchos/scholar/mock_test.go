package scholar

import (
	"context"
	"testing"

	v1 "github.com/odysseia-greek/alexandreia/aristarchos/gen/go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestRetrieveEntry_Success tests the RetrieveEntry method with a successful response
func TestRetrieveEntry_Success(t *testing.T) {
	mockService := new(MockAggregatorService)
	expectedResponse := &v1.RootWordResponse{RootWord: "λόγος", PartOfSpeech: v1.PartOfSpeech_NOUN}

	mockService.On("RetrieveEntry", mock.Anything, mock.AnythingOfType("*v1.AggregatorRequest")).Return(expectedResponse, nil)

	response, err := mockService.RetrieveEntry(context.Background(), &v1.AggregatorRequest{RootWord: "λόγος"})

	assert.NoError(t, err)
	assert.Equal(t, "λόγος", response.RootWord)
	assert.Equal(t, v1.PartOfSpeech_NOUN, response.PartOfSpeech)
	mockService.AssertCalled(t, "RetrieveEntry", mock.Anything, mock.AnythingOfType("*v1.AggregatorRequest"))
}

// TestRetrieveRootFromGrammarForm_Success tests RetrieveRootFromGrammarForm with a valid response
func TestRetrieveRootFromGrammarForm_Success(t *testing.T) {
	mockService := new(MockAggregatorService)
	expectedResponse := &v1.FormsResponse{
		Word:           "λέγω",
		UnaccentedWord: "λεγω",
		Rule:           "1st sing - pres - ind - act",
		RootWord:       "λέγω",
		Translation:    []string{"I say"},
		PartOfSpeech:   v1.PartOfSpeech_VERB.String(),
	}

	mockService.On("RetrieveRootFromGrammarForm", mock.Anything, mock.AnythingOfType("*proto.AggregatorRequest")).Return(expectedResponse, nil)

	response, err := mockService.RetrieveRootFromGrammarForm(context.Background(), &v1.AggregatorRequest{RootWord: "λέγω"})

	assert.NoError(t, err)
	assert.Equal(t, "λέγω", response.Word)
	assert.Equal(t, "1st sing - pres - ind - act", response.Rule)
	assert.ElementsMatch(t, []string{"I say"}, response.Translation)
	mockService.AssertCalled(t, "RetrieveRootFromGrammarForm", mock.Anything, mock.AnythingOfType("*proto.AggregatorRequest"))
}

// TestRetrieveSearchWords tests RetrieveSearchWords method with a successful response
func TestRetrieveSearchWords(t *testing.T) {
	mockService := new(MockAggregatorService)
	expectedResponse := &v1.SearchWordResponse{Word: []string{"λόγος", "λέγω", "λογικός"}}

	mockService.On("RetrieveSearchWords", mock.Anything, mock.AnythingOfType("*proto.AggregatorRequest")).Return(expectedResponse, nil)

	response, err := mockService.RetrieveSearchWords(context.Background(), &v1.AggregatorRequest{RootWord: "λογ"})

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"λόγος", "λέγω", "λογικός"}, response.Word)
	mockService.AssertCalled(t, "RetrieveSearchWords", mock.Anything, mock.AnythingOfType("*proto.AggregatorRequest"))
}

// TestHealthCheck_Success tests the Health method with a healthy response
func TestHealthCheck_Success(t *testing.T) {
	mockService := new(MockAggregatorService)
	expectedResponse := &v1.HealthResponse{Health: true}

	mockService.On("Health", mock.Anything, mock.AnythingOfType("*proto.HealthRequest")).Return(expectedResponse, nil)

	response, err := mockService.Health(context.Background(), &v1.HealthRequest{})

	assert.NoError(t, err)
	assert.True(t, response.Health)
	mockService.AssertCalled(t, "Health", mock.Anything, mock.AnythingOfType("*proto.HealthRequest"))
}

// TestHealthCheck_Failure tests the Health method with an unhealthy response
func TestHealthCheck_Failure(t *testing.T) {
	mockService := new(MockAggregatorService)
	expectedResponse := &v1.HealthResponse{Health: false}

	mockService.On("Health", mock.Anything, mock.AnythingOfType("*proto.HealthRequest")).Return(expectedResponse, nil)

	response, err := mockService.Health(context.Background(), &v1.HealthRequest{})

	assert.NoError(t, err)
	assert.False(t, response.Health)
	mockService.AssertCalled(t, "Health", mock.Anything, mock.AnythingOfType("*proto.HealthRequest"))
}
