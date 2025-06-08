package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterPlatformRoutes(r chi.Router) {
	r.Get("/platforms", getPlatforms)    // Assumes getPlatforms is defined in this package and takes (w, r)
	r.Post("/platforms", createPlatform) // Assumes createPlatform is defined in this package and takes (w, r)

	// Routes for specific platform items, e.g., /platforms/{platformID}
	r.Get("/platforms/{platformID}", GetPlatformByIDChiHandler)
	r.Put("/platforms/{platformID}", UpdatePlatformChiHandler)
	r.Delete("/platforms/{platformID}", DeletePlatformChiHandler)
}
