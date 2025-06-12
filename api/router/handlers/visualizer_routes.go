package handlers

import (
	"github.com/go-chi/chi/v5"
)

// RegisterVisualizationRoutes sets up the routes for visualization endpoints.
func RegisterVisualizationRoutes(r chi.Router) {
	// All routes registered here will be prefixed with /api by the main router
	// and then /visualizer by the Route group below.
	r.Route("/visualizer", func(subRouter chi.Router) {
		// Handles GET /api/visualizer/sitemap-graph
		subRouter.Get("/sitemap-graph", GetSitemapGraphDataHandler)
		// Handles GET /api/visualizer/page-sitemap-graph
		subRouter.Get("/page-sitemap-graph", GetPageSitemapGraphDataHandler)
	})
}
