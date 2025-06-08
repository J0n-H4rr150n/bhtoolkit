package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterSearchRoutes(r chi.Router) {
	// Placeholder
	r.Get("/search/traffic", SearchTrafficHandler)
}
