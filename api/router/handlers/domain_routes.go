package handlers

import (
	"github.com/go-chi/chi/v5"
)

// RegisterDomainRoutes sets up the routes for domain management.
func RegisterDomainRoutes(r chi.Router) {
	// Create a new domain (not tied to a specific target in the path, target_id in body)
	r.Post("/domains", CreateDomainHandler)

	// Get domains for a specific target
	r.Get("/targets/{target_id}/domains", GetDomainsHandler)

	// Operations on a specific domain by its ID
	r.Route("/domains/{domain_id}", func(subRouter chi.Router) {
		subRouter.Put("/", UpdateDomainHandler)
		subRouter.Delete("/", DeleteDomainHandler)
		subRouter.Get("/details", GetDomainDetailHandler)
		subRouter.Put("/favorite", SetDomainFavoriteHandler) // New route for favorite
	})

	// Discover subdomains for a specific target
	r.Post("/targets/{target_id}/domains/discover", DiscoverSubdomainsHandler)

	// Import in-scope domains from target's scope rules
	r.Post("/targets/{target_id}/domains/import-scope", ImportInScopeDomainsHandler)

	// Delete all domains for a specific target
	r.Delete("/targets/{target_id}/domains/all", DeleteAllDomainsForTargetHandler)

	// Favorite all domains matching filters for a specific target
	r.Post("/targets/{target_id}/domains/favorite-filtered", FavoriteAllFilteredDomainsHandler)

	// Run httpx for selected domains of a specific target
	r.Post("/targets/{target_id}/domains/run-httpx", RunHttpxForDomainsHandler)

	// Run httpx for ALL domains matching filters for a specific target
	r.Post("/targets/{target_id}/domains/run-httpx-all-filtered", RunHttpxForAllFilteredDomainsHandler)
}
