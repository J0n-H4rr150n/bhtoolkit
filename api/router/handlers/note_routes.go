package handlers

import (
	"net/http"
	"strconv"

	"toolkit/logger"

	"github.com/go-chi/chi/v5"
)

func RegisterNoteRoutes(r chi.Router) {
	// Collection routes for /notes
	r.Get("/notes", ListNotesHandler)   // Existing handler
	r.Post("/notes", CreateNoteHandler) // Existing handler

	// Routes for specific note items: /notes/{noteID}
	r.Route("/notes/{noteID}", func(subRouter chi.Router) {
		// GET /notes/{noteID}
		subRouter.Get("/", func(w http.ResponseWriter, req *http.Request) {
			noteIDStr := chi.URLParam(req, "noteID")
			noteID, err := strconv.ParseInt(noteIDStr, 10, 64)
			if err != nil {
				logger.Error("RegisterNoteRoutes: Invalid noteID format '%s': %v", noteIDStr, err)
				http.Error(w, "Invalid note ID format", http.StatusBadRequest)
				return
			}
			GetNoteHandler(w, req, noteID) // Existing handler
		})
		// PUT /notes/{noteID}
		subRouter.Put("/", func(w http.ResponseWriter, req *http.Request) {
			noteID, _ := strconv.ParseInt(chi.URLParam(req, "noteID"), 10, 64) // Error checked by Get
			UpdateNoteHandler(w, req, noteID)                                  // Existing handler
		})
		// DELETE /notes/{noteID}
		subRouter.Delete("/", func(w http.ResponseWriter, req *http.Request) {
			noteID, _ := strconv.ParseInt(chi.URLParam(req, "noteID"), 10, 64) // Error checked by Get
			DeleteNoteHandler(w, req, noteID)                                  // Existing handler
		})
	})
}
