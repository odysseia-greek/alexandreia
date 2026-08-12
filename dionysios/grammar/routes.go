package grammar

import (
	"net/http"

	"github.com/odysseia-greek/agora/plato/middleware"
)

// InitRoutes to start up a mux router and return the routes
func InitRoutes(dionysosHandler *DionysosHandler) *http.ServeMux {
	serveMux := http.NewServeMux()

	serveMux.HandleFunc("/healthz", middleware.Adapt(dionysosHandler.healthz, middleware.ValidateRestMethod("GET")))
	serveMux.HandleFunc("/readyz", middleware.Adapt(dionysosHandler.readyz, middleware.ValidateRestMethod("GET")))

	return serveMux
}
