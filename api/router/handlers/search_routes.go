package handlers

import (
	"net/http"
)

func RegisterSearchRoutes(mux *http.ServeMux) {
	// Placeholder
	mux.HandleFunc("GET /search/traffic", SearchTrafficHandler)
}
