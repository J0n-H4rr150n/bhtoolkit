package handlers

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// CreatePageHandler handles requests to start recording a new page.
func CreatePageHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TargetID    int64  `json:"target_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.TargetID == 0 {
		http.Error(w, "target_id is required", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	page := models.Page{
		TargetID:       req.TargetID,
		Name:           req.Name,
		Description:    sql.NullString{String: req.Description, Valid: req.Description != ""},
		StartTimestamp: time.Now(),
	}

	pageID, err := database.CreatePage(page)
	if err != nil {
		http.Error(w, "Failed to create page: "+err.Error(), http.StatusInternalServerError)
		return
	}

	page.ID = pageID
	page.CreatedAt = page.StartTimestamp // Approximately
	page.UpdatedAt = page.StartTimestamp // Approximately

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(page)
}

// StopPageRecordingHandler handles requests to stop recording a page and associate logs.
func StopPageRecordingHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PageID         int64 `json:"page_id"`
		TargetID       int64 `json:"target_id"`       // Needed to query http_traffic_log
		StartTimestamp int64 `json:"start_timestamp"` // Expect Unix timestamp (seconds)
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.PageID == 0 {
		http.Error(w, "page_id is required", http.StatusBadRequest)
		return
	}
	if req.TargetID == 0 {
		http.Error(w, "target_id is required for log association", http.StatusBadRequest)
		return
	}
	if req.StartTimestamp == 0 {
		http.Error(w, "start_timestamp is required for log association", http.StatusBadRequest)
		return
	}

	endTime := time.Now()
	startTime := time.Unix(req.StartTimestamp, 0)

	err := database.UpdatePageEndTime(req.PageID, endTime)
	if err != nil {
		http.Error(w, "Failed to update page end time: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logsAssociated, err := database.AssociateLogsToPage(req.PageID, req.TargetID, startTime, endTime)
	if err != nil {
		// Log the error, but the page end time was updated, so maybe not a 500 for the whole operation.
		// Depending on strictness, you might choose to return 500.
		logger.Error("StopPageRecordingHandler: Failed to associate logs for page ID %d: %v", req.PageID, err)
		// http.Error(w, "Failed to associate logs: "+err.Error(), http.StatusInternalServerError)
		// return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":         "Page recording stopped successfully.",
		"page_id":         req.PageID,
		"logs_associated": logsAssociated,
		"end_timestamp":   endTime,
	})
}

// GetPagesForTargetHandler retrieves all recorded pages for a target.
func GetPagesForTargetHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	if targetIDStr == "" {
		http.Error(w, "target_id query parameter is required", http.StatusBadRequest)
		return
	}
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id: "+err.Error(), http.StatusBadRequest)
		return
	}

	pages, err := database.GetPagesForTarget(targetID)
	if err != nil {
		http.Error(w, "Failed to retrieve pages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if pages == nil {
		pages = []models.Page{} // Ensure empty array instead of null for JSON
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pages)
}

// GetLogsForPageHandler retrieves all HTTP logs associated with a specific page.
func GetLogsForPageHandler(w http.ResponseWriter, r *http.Request) {
	pageIDStr := r.URL.Query().Get("page_id")
	if pageIDStr == "" {
		http.Error(w, "page_id query parameter is required", http.StatusBadRequest)
		return
	}
	pageID, err := strconv.ParseInt(pageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page_id: "+err.Error(), http.StatusBadRequest)
		return
	}
	// --- New: Parse pagination and sorting parameters ---
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 20 // Default limit
	} else if limit > 200 { // Max limit
		limit = 200
	}

	// Validate sortBy against a list of allowed column names
	// These keys should match `data-sort-key` from pageSitemapView.js
	validSortColumns := map[string]string{
		"timestamp":            "timestamp",
		"request_method":       "request_method",
		"request_url":          "request_url",
		"response_status_code": "response_status_code",
		"response_body_size":   "response_body_size",
	}
	dbSortByColumn := "timestamp" // Default sort column
	if col, ok := validSortColumns[sortBy]; ok {
		dbSortByColumn = col
	}

	dbSortOrder := "DESC" // Default sort order
	if strings.ToUpper(sortOrder) == "ASC" {
		dbSortOrder = "ASC"
	}
	// --- End New ---

	// Assume database.GetLogsForPage is updated or a new function like
	// database.GetLogsForPagePaginatedAndSorted is created.
	// It should now accept page, limit, sortBy, sortOrder and return (logs, totalRecords, error)
	logs, totalRecords, err := database.GetLogsForPagePaginatedAndSorted(pageID, page, limit, dbSortByColumn, dbSortOrder)
	if err != nil {
		logger.Error("GetLogsForPageHandler: Error fetching logs for page %d: %v", pageID, err)
		http.Error(w, "Failed to retrieve logs for page: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if logs == nil { // Ensure logs is an empty slice if null
		logs = []models.HTTPTrafficLog{}
	}

	totalPages := 0
	if totalRecords > 0 {
		totalPages = int(math.Ceil(float64(totalRecords) / float64(limit)))
	}

	response := struct {
		Logs         []models.HTTPTrafficLog `json:"logs"`
		Page         int                     `json:"page"`
		Limit        int                     `json:"limit"`
		TotalPages   int                     `json:"total_pages"`
		TotalRecords int64                   `json:"total_records"`
	}{
		Logs:         logs,
		Page:         page,
		Limit:        limit,
		TotalPages:   totalPages,
		TotalRecords: totalRecords,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("GetLogsForPageHandler: Error encoding response for page %d: %v", pageID, err)
	}
}

// DeletePageHandler handles requests to delete a specific page sitemap entry.
func DeletePageHandler(w http.ResponseWriter, r *http.Request) {
	pageIDStr := chi.URLParam(r, "page_id") // Get page_id from URL path
	if pageIDStr == "" {
		http.Error(w, "page_id path parameter is required", http.StatusBadRequest)
		return
	}

	pageID, err := strconv.ParseInt(pageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page_id in path: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = database.DeletePage(pageID)
	if err != nil {
		// Check if the error is because the page was not found, though DeletePage currently doesn't distinguish
		// For now, any error from DeletePage is treated as internal server error.
		http.Error(w, "Failed to delete page: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK) // Or http.StatusNoContent (204) if you prefer
	json.NewEncoder(w).Encode(map[string]string{"message": "Page deleted successfully"})
	logger.Info("Successfully deleted page sitemap entry with ID: %d", pageID)
}

// UpdatePagesOrderHandler handles requests to update the display order of pages.
func UpdatePagesOrderHandler(w http.ResponseWriter, r *http.Request) {
	var pageOrders map[string]int // Expecting {"pageID_string": order_int}
	if err := json.NewDecoder(r.Body).Decode(&pageOrders); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	orders := make(map[int64]int)
	for pageIDStr, order := range pageOrders {
		pageID, err := strconv.ParseInt(pageIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid page ID in payload: "+pageIDStr, http.StatusBadRequest)
			return
		}
		orders[pageID] = order
	}

	if err := database.UpdatePagesOrder(orders); err != nil {
		http.Error(w, "Failed to update page order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Page order updated successfully"})
}

// UpdatePageDetailsHandler handles requests to update the details (name, description) of a page.
func UpdatePageDetailsHandler(w http.ResponseWriter, r *http.Request) {
	pageIDStr := chi.URLParam(r, "page_id")
	if pageIDStr == "" {
		http.Error(w, "page_id path parameter is required", http.StatusBadRequest)
		return
	}
	pageID, err := strconv.ParseInt(pageIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid page_id in path: "+err.Error(), http.StatusBadRequest)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"` // Assuming frontend might send description too
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.Name == "" { // Name is typically required for a page
		http.Error(w, "Page name cannot be empty", http.StatusBadRequest)
		return
	}

	description := sql.NullString{String: req.Description, Valid: req.Description != ""}

	if err := database.UpdatePageDetails(pageID, req.Name, description); err != nil {
		http.Error(w, "Failed to update page details: "+err.Error(), http.StatusInternalServerError) // Ensure this line is complete
		return
	}

	// Optionally, fetch and return the updated page object
	// For now, just a success message.
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Page details updated successfully"})
}
