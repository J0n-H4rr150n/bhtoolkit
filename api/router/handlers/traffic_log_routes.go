package handlers

import (
	"net/http"
	"strconv"

	"toolkit/logger"

	"github.com/go-chi/chi/v5"
)

func RegisterTrafficLogRoutes(r chi.Router) {
	r.Get("/traffic-log", GetTrafficLogHandler)
	r.Get("/traffic-log/distinct-domains", GetDistinctDomainsForTargetLogsHandler)

	// Routes for specific log entries: /traffic-log/entry/{logID}
	r.Route("/traffic-log/entry/{logID}", func(subRouter chi.Router) {
		// GET /traffic-log/entry/{logID}
		subRouter.Get("/", func(w http.ResponseWriter, req *http.Request) {
			logIDStr := chi.URLParam(req, "logID")
			logID, err := strconv.ParseInt(logIDStr, 10, 64)
			if err != nil {
				logger.Error("TrafficLogEntry Get: Invalid log entry ID '%s': %v", logIDStr, err)
				http.Error(w, "Invalid log entry ID format", http.StatusBadRequest)
				return
			}
			getTrafficLogEntryDetail(w, req, logID) // Existing handler
		})

		// PUT /traffic-log/entry/{logID}/notes
		subRouter.Put("/notes", func(w http.ResponseWriter, req *http.Request) {
			logIDStr := chi.URLParam(req, "logID")
			logID, err := strconv.ParseInt(logIDStr, 10, 64)
			if err != nil {
				logger.Error("TrafficLogEntry Notes Update: Invalid log entry ID '%s': %v", logIDStr, err)
				http.Error(w, "Invalid log entry ID format", http.StatusBadRequest)
				return
			}
			updateTrafficLogEntryNotes(w, req, logID) // Existing handler
		})

		// PUT /traffic-log/entry/{logID}/favorite
		subRouter.Put("/favorite", func(w http.ResponseWriter, req *http.Request) {
			logIDStr := chi.URLParam(req, "logID")
			logID, err := strconv.ParseInt(logIDStr, 10, 64)
			if err != nil {
				logger.Error("TrafficLogEntry Favorite Update: Invalid log entry ID '%s': %v", logIDStr, err)
				http.Error(w, "Invalid log entry ID format", http.StatusBadRequest)
				return
			}
			setTrafficLogEntryFavoriteStatus(w, req, logID) // Existing handler
		})
	})

	// Route for target-specific log operations: /traffic-log/target/{targetID}
	r.Delete("/traffic-log/target/{targetID}", func(w http.ResponseWriter, req *http.Request) {
		targetIDStr := chi.URLParam(req, "targetID")
		targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			logger.Error("TrafficLogTarget Delete: Invalid target ID '%s': %v", targetIDStr, err)
			http.Error(w, "Invalid target ID format", http.StatusBadRequest)
			return
		}
		DeleteTrafficLogsForTargetHandler(w, req, targetID) // Existing handler
	})

	// Route for analyzing comments in a log entry's response body
	r.Post("/traffic-log/analyze/comments", AnalyzeCommentsHandler) // AnalyzeCommentsHandler is in traffic_log_handlers.go
}
