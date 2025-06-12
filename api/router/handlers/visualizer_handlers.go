package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"toolkit/database"
	"toolkit/logger"
)

// GetSitemapGraphDataHandler handles GET requests for the sitemap graph data.
// @Summary Get Sitemap Graph Data
// @Description Retrieves graph data (nodes and edges) for the sitemap visualization of a target.
// @Tags Visualizer
// @Produce json
// @Param target_id query int true "ID of the target"
// @Success 200 {object} models.GraphData
// @Failure 400 {object} models.ErrorResponse "Invalid or missing target_id"
// @Failure 500 {object} models.ErrorResponse "Failed to retrieve graph data"
// @Router /visualizer/sitemap-graph [get]
func GetSitemapGraphDataHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	if targetIDStr == "" {
		logger.Error("GetSitemapGraphDataHandler: target_id query parameter is required")
		http.Error(w, "target_id query parameter is required", http.StatusBadRequest)
		return
	}

	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("GetSitemapGraphDataHandler: Invalid target_id parameter '%s': %v", targetIDStr, err)
		http.Error(w, "Invalid target_id parameter, must be an integer", http.StatusBadRequest)
		return
	}

	graphData, err := database.GetSitemapGraphData(targetID)
	if err != nil {
		logger.Error("GetSitemapGraphDataHandler: Failed to get sitemap graph data for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve sitemap graph data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(graphData); err != nil {
		logger.Error("GetSitemapGraphDataHandler: Error encoding graph data to JSON: %v", err)
	}
}

// GetPageSitemapGraphDataHandler handles GET requests for the page sitemap graph data.
// @Summary Get Page Sitemap Graph Data
// @Description Retrieves graph data (nodes and edges) for the page sitemap visualization of a target.
// @Tags Visualizer
// @Produce json
// @Param target_id query int true "ID of the target"
// @Success 200 {object} models.GraphData
// @Failure 400 {object} models.ErrorResponse "Invalid or missing target_id"
// @Failure 500 {object} models.ErrorResponse "Failed to retrieve graph data"
// @Router /visualizer/page-sitemap-graph [get]
func GetPageSitemapGraphDataHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	if targetIDStr == "" {
		logger.Error("GetPageSitemapGraphDataHandler: target_id query parameter is required")
		http.Error(w, "target_id query parameter is required", http.StatusBadRequest)
		return
	}

	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("GetPageSitemapGraphDataHandler: Invalid target_id parameter '%s': %v", targetIDStr, err)
		http.Error(w, "Invalid target_id parameter, must be an integer", http.StatusBadRequest)
		return
	}

	graphData, err := database.GetPageSitemapGraphData(targetID)
	if err != nil {
		logger.Error("GetPageSitemapGraphDataHandler: Failed to get page sitemap graph data for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve page sitemap graph data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(graphData); err != nil {
		logger.Error("GetPageSitemapGraphDataHandler: Error encoding graph data to JSON: %v", err)
	}
}
