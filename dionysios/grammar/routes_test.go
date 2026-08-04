package grammar

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/odysseia-greek/agora/archytas"
	"github.com/odysseia-greek/agora/plato/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesHealthEndpoints(t *testing.T) {
	t.Run("healthz is a local liveness check", func(t *testing.T) {
		router := InitRoutes(&DionysosHandler{})
		response := performGetRequest(router, "/healthz")

		assert.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"status":"ok"}`, response.Body.String())
	})

	t.Run("readyz does not call dependencies and reports uninitialized state", func(t *testing.T) {
		router := InitRoutes(&DionysosHandler{})
		response := performGetRequest(router, "/readyz")

		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.JSONEq(t, `{"status":"not ready"}`, response.Body.String())
	})
}

func TestCheckGrammarHTTPContract(t *testing.T) {
	t.Run("missing word returns a validation response", func(t *testing.T) {
		router := InitRoutes(&DionysosHandler{})
		response := performGetRequest(router, "/dionysios/v1/checkGrammar?word=")

		var validation models.ValidationError
		require.NoError(t, json.NewDecoder(response.Body).Decode(&validation))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		require.NotEmpty(t, validation.Messages)
		assert.Equal(t, "cannot be empty", validation.Messages[0].Message)
	})

	t.Run("cached result includes the requested audit", func(t *testing.T) {
		cache, err := archytas.NewInMemoryBadgerClientWithOptions(archytas.WithLogging(false))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, cache.Close()) })

		cached := models.DeclensionTranslationResults{
			Results: []models.Result{
				{
					Word:        "λόγοι",
					Rule:        "noun - plural - masc - nom",
					RootWord:    "λόγος",
					Translation: []string{},
				},
			},
		}
		payload, err := json.Marshal(cached)
		require.NoError(t, err)
		require.NoError(t, cache.SetWithTTL("λόγοι", string(payload), time.Hour))

		router := InitRoutes(&DionysosHandler{Cache: cache})
		response := performGetRequest(router, "/dionysios/v1/checkGrammar?word=λόγοι&audit=true")

		var audited GrammarAuditResponse
		require.NoError(t, json.NewDecoder(response.Body).Decode(&audited))
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "cache", audited.Audit.DecisionSource)
		assert.Equal(t, "success", audited.Audit.Outcome)
		assert.Len(t, audited.Results, 1)
		assert.NotEmpty(t, audited.Audit.Events)
	})
}

func performGetRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
