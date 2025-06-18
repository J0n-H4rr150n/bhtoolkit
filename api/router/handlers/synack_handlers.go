package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// ListSynackTargetsHandler handles GET requests to list Synack targets.
// Supports filtering by status and an 'active_only' flag.
func ListSynackTargetsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("ListSynackTargetsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")
	sortByParam := queryParams.Get("sort_by")
	sortOrderParam := queryParams.Get("sort_order")
	activeOnlyStr := queryParams.Get("active_only")

	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	limit := 20
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		if l > 200 { // Cap limit
			limit = 200
		} else {
			limit = l
		}
	}
	offset := (page - 1) * limit

	if sortByParam == "" {
		sortByParam = "last_seen_timestamp"
	}
	if sortOrderParam == "" {
		sortOrderParam = "DESC"
	}
	sortOrderParam = strings.ToUpper(sortOrderParam)
	if sortOrderParam != "ASC" && sortOrderParam != "DESC" {
		sortOrderParam = "DESC"
	}

	var filterIsActive *bool
	if activeOnlyStr != "" {
		isActive, err := strconv.ParseBool(activeOnlyStr)
		if err == nil {
			filterIsActive = &isActive
		}
	} else {
		defaultActive := true
		filterIsActive = &defaultActive
	}

	targets, totalRecords, err := database.ListSynackTargetsPaginated(limit, offset, sortByParam, sortOrderParam, filterIsActive)
	if err != nil {
		logger.Error("ListSynackTargetsHandler: Error listing Synack targets from database: %v", err)
		http.Error(w, "Failed to list Synack targets", http.StatusInternalServerError)
		return
	}

	totalPages := int64(0)
	if totalRecords > 0 && limit > 0 {
		totalPages = int64(math.Ceil(float64(totalRecords) / float64(limit)))
	}

	response := struct {
		Targets      []models.SynackTarget `json:"targets"`
		Page         int                   `json:"page"`
		Limit        int                   `json:"limit"`
		TotalRecords int64                 `json:"total_records"`
		TotalPages   int64                 `json:"total_pages"`
	}{
		Targets:      targets,
		Page:         page,
		Limit:        limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("ListSynackTargetsHandler: Error encoding response: %v", err)
	}
}

// GetSynackTargetDetailHandler handles GET requests for a single Synack target's details.
func GetSynackTargetDetailHandler(w http.ResponseWriter, r *http.Request, targetDbID int64) {
	if r.Method != http.MethodGet {
		logger.Error("GetSynackTargetDetailHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var t models.SynackTarget
	var firstSeenStr, lastSeenStr, activatedAtStr sql.NullString
	var deactivatedAtStr sql.NullString
	var codename, name, category, status, notes, orgID, outageRaw, vulnRaw, collabRaw, rawJSON sql.NullString

	query := `SELECT id, synack_target_id_str, codename, name, category, status, is_active, 
	                 first_seen_timestamp, last_seen_timestamp, deactivated_at, notes, 
					 organization_id, activated_at, outage_windows_raw, 
					 vulnerability_discovery_raw, collaboration_criteria_raw, raw_json_details
              FROM synack_targets WHERE id = ?`

	err := database.DB.QueryRow(query, targetDbID).Scan(
		&t.DBID, &t.SynackTargetIDStr, &codename, &name, &category, &status,
		&t.IsActive, &firstSeenStr, &lastSeenStr, &deactivatedAtStr, &notes,
		&orgID, &activatedAtStr, &outageRaw, &vulnRaw, &collabRaw, &rawJSON,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			logger.Error("GetSynackTargetDetailHandler: Synack target with DB ID %d not found", targetDbID)
			http.Error(w, fmt.Sprintf("Synack target with DB ID %d not found", targetDbID), http.StatusNotFound)
		} else {
			logger.Error("GetSynackTargetDetailHandler: Error querying Synack target DB ID %d: %v", targetDbID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	if codename.Valid {
		t.Codename = codename.String
	}
	if name.Valid {
		t.Name = name.String
	}
	if category.Valid {
		t.Category = category.String
	}
	if status.Valid {
		t.Status = status.String
	}
	if notes.Valid {
		t.Notes = notes.String
	}
	if orgID.Valid {
		t.OrganizationID = orgID.String
	}
	if activatedAtStr.Valid {
		t.ActivatedAt = activatedAtStr.String
	}
	if outageRaw.Valid {
		t.OutageWindowsRaw = outageRaw.String
	}
	if vulnRaw.Valid {
		t.VulnerabilityDiscoveryRaw = vulnRaw.String
	}
	if collabRaw.Valid {
		t.CollaborationCriteriaRaw = collabRaw.String
	}
	if rawJSON.Valid {
		t.RawJSONDetails = rawJSON.String
	}

	if firstSeenStr.Valid {
		parsedTime, parseErr := time.Parse(time.RFC3339, firstSeenStr.String)
		if parseErr == nil {
			t.FirstSeenTimestamp = parsedTime
		}
	}
	if lastSeenStr.Valid {
		parsedTime, parseErr := time.Parse(time.RFC3339, lastSeenStr.String)
		if parseErr == nil {
			t.LastSeenTimestamp = parsedTime
		}
	}
	if deactivatedAtStr.Valid {
		parsedTime, parseErr := time.Parse(time.RFC3339, deactivatedAtStr.String)
		if parseErr == nil {
			t.DeactivatedAt = &parsedTime
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(t); err != nil {
		logger.Error("GetSynackTargetDetailHandler: Error encoding response for Synack target DB ID %d: %v", targetDbID, err)
	}
}

// GetSynackTargetAnalyticsHandler handles GET requests for findings of a specific Synack target.
func GetSynackTargetAnalyticsHandler(w http.ResponseWriter, r *http.Request, targetDbID int64) {
	if r.Method != http.MethodGet {
		logger.Error("GetSynackTargetAnalyticsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")
	sortByParam := queryParams.Get("sort_by")
	sortOrderParam := strings.ToUpper(queryParams.Get("sort_order"))

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 20
	} else if limit > 200 { // Cap limit
		limit = 200
	}
	offset := (page - 1) * limit

	allowedSortKeys := map[string]bool{
		"synack_finding_id": true, "title": true, "category_name": true, "severity": true,
		"status": true, "amount_paid": true, "vulnerability_url": true,
		"reported_at": true, "closed_at": true, "id": true,
	}
	dbSortColumnKey := sortByParam
	if !allowedSortKeys[sortByParam] {
		dbSortColumnKey = "reported_at"
	}

	dbSortOrder := "DESC"
	if sortOrderParam == "ASC" {
		dbSortOrder = "ASC"
	} else if sortOrderParam == "DESC" {
		dbSortOrder = "DESC"
	} else {
		if dbSortColumnKey == "title" || dbSortColumnKey == "category_name" || dbSortColumnKey == "synack_finding_id" || dbSortColumnKey == "severity" || dbSortColumnKey == "status" {
			dbSortOrder = "ASC"
		}
	}

	var targetExists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM synack_targets WHERE id = ?)", targetDbID).Scan(&targetExists)
	if err != nil {
		logger.Error("GetSynackTargetAnalyticsHandler: Error checking target existence for DB ID %d: %v", targetDbID, err)
		http.Error(w, "Internal server error verifying target", http.StatusInternalServerError)
		return
	}
	if !targetExists {
		logger.Error("GetSynackTargetAnalyticsHandler: Synack target with DB ID %d not found", targetDbID)
		http.Error(w, fmt.Sprintf("Synack target with DB ID %d not found", targetDbID), http.StatusNotFound)
		return
	}

	findings, totalRecords, err := database.GetSynackTargetFindingsPaginated(targetDbID, limit, offset, dbSortColumnKey, dbSortOrder)
	if err != nil {
		logger.Error("GetSynackTargetAnalyticsHandler: Error fetching findings for Synack target DB ID %d: %v", targetDbID, err)
		http.Error(w, "Error fetching findings data", http.StatusInternalServerError)
		return
	}

	totalPages := int64(0)
	if totalRecords > 0 && limit > 0 {
		totalPages = int64(math.Ceil(float64(totalRecords) / float64(limit)))
	}

	response := struct {
		Findings     []models.SynackFinding `json:"findings"`
		Page         int                    `json:"page"`
		Limit        int                    `json:"limit"`
		TotalRecords int64                  `json:"total_records"`
		TotalPages   int64                  `json:"total_pages"`
	}{
		Findings:     findings,
		Page:         page,
		Limit:        limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("GetSynackTargetAnalyticsHandler: Error encoding response for Synack target DB ID %d: %v", targetDbID, err)
	}
}

// ListAllSynackAnalyticsHandler handles GET requests for all Synack analytics data.
func ListAllSynackAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("ListAllSynackAnalyticsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")
	sortByParam := queryParams.Get("sort_by")
	sortOrderParam := strings.ToUpper(queryParams.Get("sort_order"))

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 20
	} else if limit > 200 { // Cap limit
		limit = 200
	}
	offset := (page - 1) * limit

	dbSortColumn := sortByParam
	if dbSortColumn == "" {
		dbSortColumn = "target_codename"
	}

	dbSortOrder := "ASC"
	if sortOrderParam == "DESC" {
		dbSortOrder = "DESC"
	}

	analytics, totalRecords, err := database.ListAllSynackAnalyticsPaginated(limit, offset, dbSortColumn, dbSortOrder)
	if err != nil {
		logger.Error("ListAllSynackAnalyticsHandler: Error fetching all analytics: %v", err)
		http.Error(w, "Error fetching analytics data", http.StatusInternalServerError)
		return
	}

	totalPages := int64(0)
	if totalRecords > 0 && limit > 0 {
		totalPages = int64(math.Ceil(float64(totalRecords) / float64(limit)))
	}

	response := struct {
		Analytics    []models.SynackGlobalAnalyticsEntry `json:"analytics"`
		Page         int                                 `json:"page"`
		Limit        int                                 `json:"limit"`
		TotalRecords int64                               `json:"total_records"`
		TotalPages   int64                               `json:"total_pages"`
	}{
		Analytics:    analytics,
		Page:         page,
		Limit:        limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("ListAllSynackAnalyticsHandler: Error encoding response: %v", err)
	}
}

// RefreshSynackTargetFindingsHandler triggers a refresh of findings for a specific Synack target.
func RefreshSynackTargetFindingsHandler(w http.ResponseWriter, r *http.Request, targetDbID int64) {
	if r.Method != http.MethodPost {
		logger.Error("RefreshSynackTargetFindingsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := database.UpdateSynackTargetAnalyticsFetchTime(targetDbID, time.Time{}); err != nil {
		logger.Error("RefreshSynackTargetFindingsHandler: Error resetting analytics fetch time for target %d: %v", targetDbID, err)
		http.Error(w, "Failed to initiate findings refresh", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "Findings refresh initiated. The data may take a few minutes to update."})
}

// ListObservedMissionsHandler handles GET requests to list observed Synack missions.
func ListObservedMissionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("ListObservedMissionsHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queryParams := r.URL.Query()
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")
	sortByParam := queryParams.Get("sort_by")
	sortOrderParam := strings.ToUpper(queryParams.Get("sort_order"))

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 200 { // Cap limit
		limit = 50 // Default limit for missions
	}
	offset := (page - 1) * limit

	// Define allowed sort keys for missions
	allowedSortKeys := map[string]bool{"id": true, "title": true, "payout_amount": true, "status": true, "claimed_by_toolkit_at": true, "created_at": true, "updated_at": true}
	dbSortColumnKey := sortByParam
	if !allowedSortKeys[sortByParam] {
		dbSortColumnKey = "created_at" // Default sort
	}

	dbSortOrder := "DESC" // Default sort order
	if sortOrderParam == "ASC" {
		dbSortOrder = "ASC"
	}

	missions, totalRecords, err := database.ListObservedMissionsPaginated(limit, offset, dbSortColumnKey, dbSortOrder)
	if err != nil {
		logger.Error("ListObservedMissionsHandler: Error listing observed missions: %v", err)
		http.Error(w, "Failed to list observed missions", http.StatusInternalServerError)
		return
	}

	response := models.PaginatedResponse{ // Using the generic paginated response
		Page:         page,
		Limit:        limit,
		TotalRecords: int(totalRecords), // Cast totalRecords to int for generic struct
		TotalPages:   int((totalRecords + int64(limit) - 1) / int64(limit)),
		Records:      missions,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("ListObservedMissionsHandler: Error encoding response: %v", err)
	}
}
