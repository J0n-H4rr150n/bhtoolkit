package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterSitemapRoutes(r chi.Router) {
	r.Get("/sitemap/manual-entries", GetSitemapManualEntriesHandler) // New route
	r.Get("/sitemap-endpoints", GetSitemapEndpointsHandler)
	r.Get("/endpoint-instances", GetEndpointInstancesHandler)
	r.Post("/sitemap/manual-entry", AddSitemapManualEntryHandler)
}
