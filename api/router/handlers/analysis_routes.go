package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterAnalysisRoutes(r chi.Router) {
	r.Post("/analyze/jslinks", AnalyzeJSLinksHandler)
}
