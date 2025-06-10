package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterSitemapRoutes(r chi.Router) {
	r.Get("/sitemap/manual-entries", GetSitemapManualEntriesHandler)
	r.Get("/sitemap/generated", GetGeneratedSitemapHandler) // New route for the tree
	r.Get("/sitemap-endpoints", GetSitemapEndpointsHandler)
	r.Get("/endpoint-instances", GetEndpointInstancesHandler)
	r.Post("/sitemap/manual-entry", AddSitemapManualEntryHandler)

	// Routes for Page Sitemap feature
	r.Post("/pages", CreatePageHandler)                 // Create a new page recording
	r.Post("/pages/stop", StopPageRecordingHandler)     // Stop recording and associate logs
	r.Get("/pages", GetPagesForTargetHandler)           // Get all pages for a target_id
	r.Get("/pages/logs", GetLogsForPageHandler)         // Get all logs for a page_id
	r.Put("/pages/order", UpdatePagesOrderHandler)      // Update display order of pages
	r.Put("/pages/{page_id}", UpdatePageDetailsHandler) // Update page details (name, description)
	r.Delete("/pages/{page_id}", DeletePageHandler)     // Delete a specific page

	r.Post("/targets/{target_id}/analyze-parameters", AnalyzeTargetForParameterizedURLsHandler)
	r.Get("/parameterized-urls", GetParameterizedURLsHandler)
}
