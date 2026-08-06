package grammar

import (
	"net/http"

	"github.com/odysseia-greek/agora/plato/middleware"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
)

// InitRoutes to start up a mux router and return the routes
func InitRoutes(dionysosHandler *DionysosHandler) *http.ServeMux {
	serveMux := http.NewServeMux()

	serveMux.HandleFunc("/healthz", middleware.Adapt(dionysosHandler.healthz, middleware.ValidateRestMethod("GET")))
	serveMux.HandleFunc("/readyz", middleware.Adapt(dionysosHandler.readyz, middleware.ValidateRestMethod("GET")))
	serveMux.HandleFunc("/dionysios/v1/checkGrammar", middleware.Adapt(dionysosHandler.checkGrammar, middleware.ValidateRestMethod("GET"), middleware.Adapter(comedy.TraceWithHopStop(dionysosHandler.Streamer))))
	serveMux.HandleFunc("/dionysios/v1/research", middleware.Adapt(dionysosHandler.researchWord, middleware.ValidateRestMethod("POST"), middleware.Adapter(comedy.TraceWithHopStop(dionysosHandler.Streamer))))

	return serveMux
}
