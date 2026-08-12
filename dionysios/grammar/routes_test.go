package grammar

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestLegacyHTTPAPIIsNotExposed(t *testing.T) {
	router := InitRoutes(&DionysosHandler{})

	for _, path := range []string{
		"/dionysios/v1/checkGrammar?word=λόγοι",
		"/dionysios/v1/research",
	} {
		response := performGetRequest(router, path)
		assert.Equal(t, http.StatusNotFound, response.Code, path)
	}
}

func performGetRequest(handler http.Handler, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
