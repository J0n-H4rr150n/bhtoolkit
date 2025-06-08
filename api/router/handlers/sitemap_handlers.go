package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// GetSitemapEndpointsHandler is a placeholder for getting unique sitemap endpoints.
// @Summary Get sitemap/unique endpoints for a target
// @Description (Not Implemented Yet) Retrieves a list of unique [METHOD] /path combinations discovered for a target.
// @Tags Sitemap
// @Produce json
// @Param target_id query int true "ID of the target"
// @Success 501 {object} models.ErrorResponse "Not Implemented Yet"
// @Failure 400 {object} models.ErrorResponse "Invalid or missing target_id"
// @Router /sitemap-endpoints [get]
func GetSitemapEndpointsHandler(w http.ResponseWriter, r *http.Request) {
	notImplementedHandler(w, r)
}

// GetEndpointInstancesHandler is a placeholder for getting all instances of a specific endpoint.
// @Summary Get all instances for a specific sitemap endpoint
// @Description (Not Implemented Yet) Retrieves all logged HTTP requests that match a specific [METHOD] /path combination for a target.
// @Tags Sitemap
// @Produce json
// @Param target_id query int true "ID of the target"
// @Param method query string true "HTTP method of the endpoint (e.g., GET, POST)"
// @Param path query string true "Normalized URL path of the endpoint (e.g., /api/users)"
// @Param page query int false "Page number for pagination" default(1)
// @Param limit query int false "Number of items per page" default(50)
// @Success 501 {object} models.ErrorResponse "Not Implemented Yet"
// @Failure 400 {object} models.ErrorResponse "Missing or invalid parameters"
// @Router /endpoint-instances [get]
func GetEndpointInstancesHandler(w http.ResponseWriter, r *http.Request) {
	notImplementedHandler(w, r)
}

// GetSitemapManualEntriesHandler handles GET requests to list manual sitemap entries for a target.
func GetSitemapManualEntriesHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	if targetIDStr == "" {
		logger.Error("GetSitemapManualEntriesHandler: target_id query parameter is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "target_id query parameter is required"})
		return
	}

	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("GetSitemapManualEntriesHandler: Invalid target_id parameter '%s': %v", targetIDStr, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid target_id parameter, must be an integer"})
		return
	}

	entries, err := database.GetSitemapManualEntriesByTargetID(targetID)
	if err != nil {
		logger.Error("GetSitemapManualEntriesHandler: Error fetching sitemap manual entries for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve sitemap manual entries", http.StatusInternalServerError)
		return
	}

	if entries == nil { // Ensure we return an empty array, not null
		entries = []models.SitemapManualEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// AddSitemapManualEntryHandler handles POST requests to add a manual entry to the sitemap.
func AddSitemapManualEntryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("AddSitemapManualEntryHandler: MethodNotAllowed: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed. Only POST is accepted."})
		return
	}

	var req models.AddSitemapManualEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("AddSitemapManualEntryHandler: Error decoding request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request payload: " + err.Error()})
		return
	}
	defer r.Body.Close()

	if req.HTTPTrafficLogID == 0 {
		logger.Error("AddSitemapManualEntryHandler: http_log_id is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "http_log_id is required"})
		return
	}
	req.FolderPath = strings.TrimSpace(req.FolderPath)
	if req.FolderPath == "" {
		logger.Error("AddSitemapManualEntryHandler: folder_path is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "folder_path is required"})
		return
	}
	// Ensure folder_path starts and ends with a slash for consistency, unless it's just "/"
	if !strings.HasPrefix(req.FolderPath, "/") {
		req.FolderPath = "/" + req.FolderPath
	}
	if len(req.FolderPath) > 1 && !strings.HasSuffix(req.FolderPath, "/") {
		req.FolderPath += "/"
	}

	// Fetch original log entry details
	var originalLog struct {
		TargetID      sql.NullInt64
		RequestMethod string
		RequestURL    string
	}
	err := database.DB.QueryRow("SELECT target_id, request_method, request_url FROM http_traffic_log WHERE id = ?", req.HTTPTrafficLogID).Scan(
		&originalLog.TargetID, &originalLog.RequestMethod, &originalLog.RequestURL,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("AddSitemapManualEntryHandler: Original http_traffic_log entry with ID %d not found", req.HTTPTrafficLogID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Original log entry with ID %d not found", req.HTTPTrafficLogID)})
		} else {
			logger.Error("AddSitemapManualEntryHandler: Error fetching original log entry ID %d: %v", req.HTTPTrafficLogID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to retrieve original log entry details"})
		}
		return
	}

	if !originalLog.TargetID.Valid {
		logger.Error("AddSitemapManualEntryHandler: Original log entry ID %d is not associated with a target (target_id is NULL)", req.HTTPTrafficLogID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Original log entry is not associated with a target"})
		return
	}

	parsedURL, err := url.Parse(originalLog.RequestURL)
	if err != nil {
		logger.Error("AddSitemapManualEntryHandler: Could not parse RequestURL '%s' from log ID %d: %v", originalLog.RequestURL, req.HTTPTrafficLogID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to parse URL from original log entry"})
		return
	}
	requestPath := parsedURL.Path

	sitemapEntry := models.SitemapManualEntry{
		TargetID:         originalLog.TargetID.Int64,
		HTTPTrafficLogID: sql.NullInt64{Int64: req.HTTPTrafficLogID, Valid: true},
		FolderPath:       req.FolderPath,
		RequestMethod:    originalLog.RequestMethod,
		RequestPath:      requestPath,
		Notes:            models.NullString(req.Notes),
	}

	id, err := database.CreateSitemapManualEntry(sitemapEntry) // This function needs to be created in database package
	if err != nil {
		// database.CreateSitemapManualEntry should handle logging specific SQL errors
		// It might return a specific error type for duplicates (UNIQUE constraint on http_traffic_log_id)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			logger.Info("AddSitemapManualEntryHandler: Attempt to add duplicate sitemap entry for http_log_id %d. Error: %v", req.HTTPTrafficLogID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "This log entry has already been added to the sitemap."})
		} else {
			logger.Error("AddSitemapManualEntryHandler: Error creating sitemap manual entry: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to add entry to sitemap."})
		}
		return
	}
	sitemapEntry.ID = id
	// sitemapEntry.CreatedAt and UpdatedAt will be set by DB default or by CreateSitemapManualEntry

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sitemapEntry)
	logger.Info("Successfully added manual sitemap entry ID %d for log ID %d, folder '%s'", id, req.HTTPTrafficLogID, req.FolderPath)
}
