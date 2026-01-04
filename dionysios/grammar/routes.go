package grammar

import (
	"github.com/gorilla/mux"
	"github.com/odysseia-greek/agora/plato/middleware"
	"github.com/odysseia-greek/attike/aristophanes/comedy"
)

// InitRoutes to start up a mux router and return the routes
func InitRoutes(dionysosHandler *DionysosHandler) *mux.Router {
	serveMux := mux.NewRouter()

	serveMux.HandleFunc("/dionysios/v1/ping", middleware.Adapt(dionysosHandler.pingPong, middleware.ValidateRestMethod("GET")))
	serveMux.HandleFunc("/dionysios/v1/health", middleware.Adapt(dionysosHandler.health, middleware.ValidateRestMethod("GET")))
	serveMux.HandleFunc("/dionysios/v1/checkGrammar", middleware.Adapt(dionysosHandler.checkGrammar, middleware.ValidateRestMethod("GET"), middleware.Adapter(comedy.TraceWithLogAndSpan(dionysosHandler.Streamer))))

	return serveMux
}
