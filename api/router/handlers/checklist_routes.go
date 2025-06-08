package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func RegisterChecklistRoutes(r chi.Router) {
	// POST /checklist-items
	// The AddChecklistItemHandler already takes (w, r) and reads target_id from the body.
	r.Post("/checklist-items", AddChecklistItemHandler)

	// Routes for specific checklist items: /checklist-items/{itemID}
	r.Route("/checklist-items/{itemID}", func(subRouter chi.Router) {
		// PUT /checklist-items/{itemID}
		subRouter.Put("/", func(w http.ResponseWriter, req *http.Request) {
			itemIDStr := chi.URLParam(req, "itemID")
			itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid checklist item ID", http.StatusBadRequest)
				return
			}
			UpdateChecklistItemHandler(w, req, itemID) // Existing handler
		})
		// DELETE /checklist-items/{itemID}
		subRouter.Delete("/", func(w http.ResponseWriter, req *http.Request) {
			itemIDStr := chi.URLParam(req, "itemID")
			itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid checklist item ID", http.StatusBadRequest)
				return
			}
			DeleteChecklistItemHandler(w, req, itemID) // Existing handler
		})
	})
}
