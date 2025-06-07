package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"toolkit/logger"
)

func RegisterTargetRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/targets", TargetsHandler)    // Handles GET and POST for /targets
	mux.HandleFunc("/target/", targetItemHandler) // Dispatcher for /target/{idOrSlug} and /target/{id}/checklist-items
	mux.HandleFunc("DELETE /targets/by-codename", DeleteTargetByCodenameHandler)
	mux.HandleFunc("POST /targets/from-synack", PromoteSynackTargetHandler)
}

// targetItemHandler dispatches requests for /target/{identifier} and /target/{target_id}/checklist-items.
func targetItemHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/target/")
	parts := strings.SplitN(path, "/", 2)
	idOrSlug := parts[0]

	if len(parts) == 2 && parts[1] == "checklist-items" {
		targetID, err := strconv.ParseInt(idOrSlug, 10, 64)
		if err != nil {
			http.Error(w, "Invalid target ID for checklist items", http.StatusBadRequest)
			return
		}
		if r.Method == http.MethodGet {
			GetChecklistItemsHandler(w, r, targetID)
		} else {
			http.Error(w, "Method not allowed for target checklist items", http.StatusMethodNotAllowed)
		}
		return
	}
	// If not checklist-items, then it's a regular target item operation
	originalTargetItemHandler(w, r, idOrSlug)
}

// originalTargetItemHandler handles GET, PUT, DELETE for a specific target by ID or Slug.
func originalTargetItemHandler(w http.ResponseWriter, r *http.Request, idOrSlug string) {
	switch r.Method {
	case http.MethodGet:
		targetID, err := strconv.ParseInt(idOrSlug, 10, 64)
		if err != nil {
			// TODO: If not a valid int, it might be a slug.
			// The GetTargetByID function (or its equivalent for slugs) should handle this.
			// For now, we assume GetTargetByID can handle slugs or this needs adjustment.
			logger.Error("originalTargetItemHandler: GET expects a numeric ID for now. Received: %s", idOrSlug)
			http.Error(w, "Invalid target ID for GET (expected numeric)", http.StatusBadRequest)
			return
		}
		GetTargetByID(w, r, targetID)
	case http.MethodPut:
		targetID, err := strconv.ParseInt(idOrSlug, 10, 64)
		if err != nil {
			http.Error(w, "Invalid target ID for update (expected numeric)", http.StatusBadRequest)
			return
		}
		UpdateTargetDetailsHandler(w, r, targetID)
	case http.MethodDelete:
		DeleteTarget(w, r, idOrSlug)
	default:
		http.Error(w, "Method not allowed for /target/{identifier}", http.StatusMethodNotAllowed)
	}
}
