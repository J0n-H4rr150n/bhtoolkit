package handlers

import (
	"github.com/go-chi/chi/v5"
)

func RegisterVersionRoutes(r chi.Router) {
	r.Get("/version", GetVersionHandler)
}
