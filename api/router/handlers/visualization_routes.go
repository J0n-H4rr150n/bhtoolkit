package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterVisualizationRoutes(r chi.Router) {
	// Placeholder
	r.Get("/visualization", GetVisualizationDataHandler)
}
