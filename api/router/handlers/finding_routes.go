package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterFindingRoutes(r chi.Router) {
	// Assuming findings are often nested under targets
	r.Post("/targets/{target_id}/findings", CreateTargetFindingHandler)
	r.Get("/targets/{target_id}/findings", GetTargetFindingsHandler) // This is the one we need for GET

	// Routes for individual findings (update, delete)
	r.Put("/findings/{finding_id}", UpdateTargetFindingHandler)
	r.Delete("/findings/{finding_id}", DeleteTargetFindingHandler)
	// GET /findings/{finding_id} could also be added if needed directly
}
