package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

func RegisterChecklistTemplateRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /checklist-templates", ListChecklistTemplatesHandler)
	mux.HandleFunc("/checklist-templates/", func(w http.ResponseWriter, r *http.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/checklist-templates/")
		parts := strings.SplitN(trimmedPath, "/", 2)
		templateIDStr := parts[0]

		templateID, err := strconv.ParseInt(templateIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid checklist template ID format", http.StatusBadRequest)
			return
		}

		if len(parts) == 2 && parts[1] == "items" && r.Method == http.MethodGet {
			GetChecklistTemplateItemsHandler(w, r, templateID)
		} else {
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("POST /checklist-templates/copy-to-target", CopyTemplateItemsToTargetHandler)
}
