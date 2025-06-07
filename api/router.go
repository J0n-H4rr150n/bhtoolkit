package api

import (
	"net/http"
	"toolkit/api/router/handlers"
	"toolkit/logger"
)

// NewRouter creates and configures a new HTTP ServeMux for the API.
// All registered paths are relative to the /api base path.
func NewRouter() http.Handler {
	router := http.NewServeMux()

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
	handlers.RegisterNoteRoutes(router)

	// Placeholder/Not Implemented Yet routes
	handlers.RegisterRelationshipRoutes(router)
	handlers.RegisterVisualizationRoutes(router)
	handlers.RegisterSearchRoutes(router)
	handlers.RegisterSitemapRoutes(router)

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		logger.Error("API SUB-ROUTER CATCH-ALL: Unhandled route relative to /api: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	return router
}
