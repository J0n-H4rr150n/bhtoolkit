package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterRelationshipRoutes(r chi.Router) {
	r.Post("/relationships/create", CreateRelationshipHandler)
}
