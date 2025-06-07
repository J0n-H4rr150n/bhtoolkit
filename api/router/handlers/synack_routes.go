package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"toolkit/logger"
)

func RegisterSynackRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /synack-targets", ListSynackTargetsHandler)
	mux.HandleFunc("/synack-targets/", func(w http.ResponseWriter, r *http.Request) {
		const basePath = "/synack-targets/"
		// This check ensures that requests like "/synack-targets/" (with trailing slash but no ID) are caught.
		if r.URL.Path == basePath || r.URL.Path == strings.TrimRight(basePath, "/") {
			http.NotFound(w, r) // Or specific error for missing ID
			return
		}
		trimmedPath := strings.TrimPrefix(r.URL.Path, basePath)
		parts := strings.SplitN(trimmedPath, "/", 2)
		idStr := parts[0]

		targetDbID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			logger.Error("Synack Target Item Router: Invalid target_db_id '%s': %v", idStr, err)
			http.Error(w, "Invalid Synack target DB ID format", http.StatusBadRequest)
			return
		}

		if len(parts) == 2 { // Path has a sub-resource like /synack-targets/{id}/analytics
			switch parts[1] {
			case "analytics":
				if r.Method == http.MethodGet {
					GetSynackTargetAnalyticsHandler(w, r, targetDbID)
				} else {
					http.Error(w, "Method not allowed for analytics sub-resource", http.StatusMethodNotAllowed)
				}
			case "refresh":
				if r.Method == http.MethodPost {
					RefreshSynackTargetFindingsHandler(w, r, targetDbID)
				} else {
					http.Error(w, "Method not allowed for refresh sub-resource", http.StatusMethodNotAllowed)
				}
			default:
				http.NotFound(w, r)
			}
		} else if len(parts) == 1 && r.Method == http.MethodGet { // Path is just /synack-targets/{id}
			GetSynackTargetDetailHandler(w, r, targetDbID)
		} else { // Path is malformed or method not allowed for /synack-targets/{id}
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("GET /synack-analytics/all", ListAllSynackAnalyticsHandler)
}
