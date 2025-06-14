package api

import (
	"net/http"
	"toolkit/api/router/handlers"
	"toolkit/logger"

	"github.com/go-chi/chi/v5"
)

// NewRouter creates and configures a new HTTP ServeMux for the API.
// All registered paths are relative to the /api base path.
func NewRouter() http.Handler {
	router := chi.NewRouter()

	handlers.RegisterHealthRoutes(router)
	handlers.RegisterPlatformRoutes(router)
	handlers.RegisterTargetRoutes(router)
	handlers.RegisterScopeRuleRoutes(router)
	handlers.RegisterSynackRoutes(router)
	handlers.RegisterTrafficLogRoutes(router)
	handlers.RegisterAnalysisRoutes(router)
	handlers.RegisterSettingsRoutes(router)
	handlers.RegisterChecklistRoutes(router)
	handlers.RegisterChecklistTemplateRoutes(router)
	handlers.RegisterFindingRoutes(router)
	handlers.RegisterNoteRoutes(router)
	handlers.RegisterModifierRoutes(router)
	handlers.RegisterProxySendRoutes(router) // New line to register proxy send handler
	handlers.RegisterVersionRoutes(router)   // Add version routes

	// Placeholder/Not Implemented Yet routes
	handlers.RegisterRelationshipRoutes(router)
	handlers.RegisterVisualizationRoutes(router)
	handlers.RegisterSearchRoutes(router)
	handlers.RegisterSitemapRoutes(router) // This will also need to be adapted for chi.Router

	// Catch-all for unhandled routes within the API
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		logger.Error("API SUB-ROUTER CATCH-ALL: Unhandled route relative to /api: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	return router
}
