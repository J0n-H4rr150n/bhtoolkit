package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterTargetRoutes(r chi.Router) {
	// Collection routes for /targets
	r.Get("/targets", getTargets)    // Assumes getTargets is defined in platform_handlers.go or target_handlers.go
	r.Post("/targets", createTarget) // Assumes createTarget is defined

	// Routes for specific target items, e.g., /target/{idOrSlug}
	// We use idOrSlug because delete can take a slug, while GET/PUT usually take ID.
	// The handlers will need to differentiate if necessary.
	r.Route("/target/{idOrSlug}", func(subRouter chi.Router) {
		// GET /target/{idOrSlug}
		subRouter.Get("/", GetTargetByIDChiHandler) // New handler to be created
		// PUT /target/{idOrSlug}
		subRouter.Put("/", UpdateTargetDetailsChiHandler) // New handler to be created
		// DELETE /target/{idOrSlug}
		subRouter.Delete("/", DeleteTargetChiHandler) // New handler to be created

		// Nested route for checklist items: /target/{targetID}/checklist-items
		// Note: {idOrSlug} here should resolve to a numeric targetID for checklist items.
		// The GetChecklistItemsForTargetChiHandler will need to parse it as int.
		subRouter.Get("/checklist-items", GetChecklistItemsForTargetChiHandler) // New handler to be created
	})

	// Specific operational routes
	r.Delete("/targets/by-codename", DeleteTargetByCodenameHandler) // Existing handler
	r.Post("/targets/from-synack", PromoteSynackTargetHandler)      // Existing handler
}
