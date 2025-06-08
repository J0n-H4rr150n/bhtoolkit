package handlers

import (
	"net/http"
	"strconv"
	"toolkit/logger"

	"github.com/go-chi/chi/v5"
)

func RegisterSynackRoutes(r chi.Router) {
	r.Get("/synack-targets", ListSynackTargetsHandler)

	r.Route("/synack-targets/{targetDbID}", func(subRouter chi.Router) {
		// GET /synack-targets/{targetDbID}
		subRouter.Get("/", func(w http.ResponseWriter, req *http.Request) {
			targetDbIDStr := chi.URLParam(req, "targetDbID")
			targetDbID, err := strconv.ParseInt(targetDbIDStr, 10, 64)
			if err != nil {
				logger.Error("Synack Target Item Router: Invalid target_db_id '%s': %v", targetDbIDStr, err)
				http.Error(w, "Invalid Synack target DB ID format", http.StatusBadRequest)
				return
			}
			GetSynackTargetDetailHandler(w, req, targetDbID)
		})

		// GET /synack-targets/{targetDbID}/analytics
		subRouter.Get("/analytics", func(w http.ResponseWriter, req *http.Request) {
			targetDbIDStr := chi.URLParam(req, "targetDbID")
			targetDbID, _ := strconv.ParseInt(targetDbIDStr, 10, 64) // Error handled by outer route if needed
			GetSynackTargetAnalyticsHandler(w, req, targetDbID)
		})
		// POST /synack-targets/{targetDbID}/refresh
		subRouter.Post("/refresh", func(w http.ResponseWriter, req *http.Request) {
			targetDbIDStr := chi.URLParam(req, "targetDbID")
			targetDbID, _ := strconv.ParseInt(targetDbIDStr, 10, 64) // Error handled by outer route if needed
			RefreshSynackTargetFindingsHandler(w, req, targetDbID)
		})
	})
	r.Get("/synack-analytics/all", ListAllSynackAnalyticsHandler)
}
