package handlers

import (
	"github.com/go-chi/chi/v5"
)

// RegisterTagRoutes sets up the routes for tag management.
func RegisterTagRoutes(r chi.Router) {
	// Routes for general tag operations (e.g., GET all, POST new)
	r.Route("/tags", func(subRouter chi.Router) {
		subRouter.Get("/", ListTagsHandler)   // Assumes ListTagsHandler exists or will be created
		subRouter.Post("/", CreateTagHandler) // Assumes CreateTagHandler exists or will be created
	})

	// Routes for specific tag operations by ID
	r.Route("/tags/{tagID}", func(subRouter chi.Router) {
		subRouter.Get("/", GetTagByIDHandler)   // Assumes GetTagByIDHandler exists or will be created
		subRouter.Put("/", UpdateTagHandler)    // Handler created in the previous step
		subRouter.Delete("/", DeleteTagHandler) // Handler created in the previous step
	})

	// Routes for tag associations
	r.Post("/tag-associations", AssociateTagHandler)      // Assumes AssociateTagHandler exists or will be created
	r.Delete("/tag-associations", DisassociateTagHandler) // Assumes DisassociateTagHandler exists or will be created
}
