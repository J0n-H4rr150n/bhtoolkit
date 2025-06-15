package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterSubfinderRoutes(r chi.Router) {
	// GET /api/subfinder/status
	r.Get("/subfinder/status", GetSubfinderStatusHandler)
}
