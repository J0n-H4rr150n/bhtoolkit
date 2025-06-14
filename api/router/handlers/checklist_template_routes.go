package handlers

import (
	"net/http"
	"strconv"

	"toolkit/logger" // Assuming logger is used or might be useful

	"github.com/go-chi/chi/v5"
)

func RegisterChecklistTemplateRoutes(r chi.Router) {
	// GET /checklist-templates
	r.Get("/checklist-templates", ListChecklistTemplatesHandler) // Existing handler

	// GET /checklist-templates/{templateID}/items
	r.Get("/checklist-templates/{templateID}/items", func(w http.ResponseWriter, req *http.Request) {
		templateIDStr := chi.URLParam(req, "templateID")
		templateID, err := strconv.ParseInt(templateIDStr, 10, 64)
		if err != nil {
			logger.Error("RegisterChecklistTemplateRoutes: Invalid templateID format '%s': %v", templateIDStr, err)
			http.Error(w, "Invalid checklist template ID format", http.StatusBadRequest)
			return
		}
		GetChecklistTemplateItemsHandler(w, req, templateID) // Existing handler
	})

	// POST /checklist-templates/copy-to-target
	r.Post("/checklist-templates/copy-to-target", CopyTemplateItemsToTargetHandler) // Existing handler

	// POST /checklist-templates/copy-all-to-target
	r.Post("/checklist-templates/copy-all-to-target", CopyAllChecklistTemplateItemsToTargetHandler) // New handler
}
