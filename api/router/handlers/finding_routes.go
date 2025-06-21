package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterFindingRoutes(r chi.Router) {
	// Assuming findings are often nested under targets
	r.Post("/findings", CreateTargetFindingHandler)                  // Changed: Create finding, target_id in body
	r.Get("/targets/{target_id}/findings", GetTargetFindingsHandler) // List findings for a specific target

	// Routes for individual findings (get by ID, update, delete)
	r.Route("/findings/{finding_id}", func(subRouter chi.Router) {
		subRouter.Get("/", GetFindingByIDHandler) // Added: Get a specific finding by its ID
		subRouter.Put("/", UpdateTargetFindingHandler)
		subRouter.Delete("/", DeleteTargetFindingHandler)
	})

	// Routes for Vulnerability Types
	r.Post("/vulnerability-types", CreateVulnerabilityTypeHandler)
	r.Get("/vulnerability-types", GetAllVulnerabilityTypesHandler)
	r.Route("/vulnerability-types/{vulnerability_type_id}", func(subRouter chi.Router) {
		subRouter.Put("/", UpdateVulnerabilityTypeHandler)
		subRouter.Delete("/", DeleteVulnerabilityTypeHandler)
	})
}
