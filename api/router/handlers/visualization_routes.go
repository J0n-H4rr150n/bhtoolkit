package handlers

import (
	"net/http"
)

func RegisterVisualizationRoutes(mux *http.ServeMux) {
	// Placeholder
	mux.HandleFunc("GET /visualization", GetVisualizationDataHandler)
}
