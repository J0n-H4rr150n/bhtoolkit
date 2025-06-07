package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// GetTrafficLogHandler retrieves paginated and filtered HTTP traffic log entries for a target.
// It supports filtering by various fields like method, status, content type, and a general search term.
// It also provides distinct values for filter dropdowns in the UI.
func GetTrafficLogHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		notImplementedHandler(w, r) // Assuming notImplementedHandler is in the same 'handlers' package
		return
	}

	targetIDStr := r.URL.Query().Get("target_id")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	sortByParam := r.URL.Query().Get("sort_by")
	sortOrderParam := strings.ToUpper(r.URL.Query().Get("sort_order"))
	favoritesOnlyStr := r.URL.Query().Get("favorites_only")
	filterMethod := strings.ToUpper(r.URL.Query().Get("method"))
	filterStatus := r.URL.Query().Get("status")
	filterContentType := r.URL.Query().Get("type")
	filterSearchText := r.URL.Query().Get("search")

	if targetIDStr == "" {
		logger.Error("GetTrafficLogHandler: target_id query parameter is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "target_id query parameter is required"})
		return
	}

	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Invalid target_id parameter '%s': %v", targetIDStr, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid target_id parameter, must be an integer"})
		return
	}

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 20
	} else if limit > 200 { // Cap the limit to prevent DoS
		limit = 200
	}
	offset := (page - 1) * limit

	whereClauses := []string{"target_id = ?"}
	queryArgs := []interface{}{targetID}

	distinctWhereClauses := []string{"target_id = ?"}
	distinctQueryArgs := []interface{}{targetID}

	if favOnly, err := strconv.ParseBool(favoritesOnlyStr); err == nil && favOnly {
		whereClauses = append(whereClauses, "is_favorite = TRUE")
		distinctWhereClauses = append(distinctWhereClauses, "is_favorite = TRUE")
	}

	if filterMethod != "" {
		whereClauses = append(whereClauses, "UPPER(request_method) = ?")
		queryArgs = append(queryArgs, filterMethod)
	}

	if filterStatus != "" {
		statusCode, err := strconv.ParseInt(filterStatus, 10, 64)
		if err == nil {
			whereClauses = append(whereClauses, "response_status_code = ?")
			queryArgs = append(queryArgs, statusCode)
		} else {
			logger.Info("GetTrafficLogHandler: Invalid status code filter '%s': %v. Ignoring filter.", filterStatus, err)
		}
	}

	if filterContentType != "" {
		whereClauses = append(whereClauses, "LOWER(response_content_type) LIKE LOWER(?)")
		queryArgs = append(queryArgs, "%"+filterContentType+"%")
	}

	if filterSearchText != "" {
		searchClause := `(LOWER(request_url) LIKE LOWER(?) OR UPPER(request_method) LIKE UPPER(?) OR LOWER(response_content_type) LIKE LOWER(?) OR CAST(response_status_code AS TEXT) LIKE ?)`
		whereClauses = append(whereClauses, searchClause)
		distinctWhereClauses = append(distinctWhereClauses, searchClause)
		searchPattern := "%" + filterSearchText + "%"
		queryArgs = append(queryArgs, searchPattern, searchPattern, searchPattern, searchPattern)
		distinctQueryArgs = append(distinctQueryArgs, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	finalWhereClause := strings.Join(whereClauses, " AND ")
	finalDistinctWhereClause := strings.Join(distinctWhereClauses, " AND ")
	distinctValues := make(map[string][]string)

	methodQuery := fmt.Sprintf("SELECT DISTINCT request_method FROM http_traffic_log WHERE %s ORDER BY request_method ASC", finalDistinctWhereClause)
	rows, err := database.DB.Query(methodQuery, distinctQueryArgs...)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Error fetching distinct methods for target %d: %v", targetID, err)
	} else {
		defer rows.Close()
		var method string
		for rows.Next() {
			if err := rows.Scan(&method); err == nil {
				distinctValues["method"] = append(distinctValues["method"], method)
			}
		}
		if err = rows.Err(); err != nil {
			logger.Error("GetTrafficLogHandler: Error iterating distinct methods: %v", err)
		}
	}

	statusQuery := fmt.Sprintf("SELECT DISTINCT response_status_code FROM http_traffic_log WHERE %s ORDER BY response_status_code ASC", finalDistinctWhereClause)
	rows, err = database.DB.Query(statusQuery, distinctQueryArgs...)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Error fetching distinct statuses for target %d: %v", targetID, err)
	} else {
		defer rows.Close()
		var status sql.NullInt64
		for rows.Next() {
			if err := rows.Scan(&status); err == nil && status.Valid {
				distinctValues["status"] = append(distinctValues["status"], strconv.FormatInt(status.Int64, 10))
			}
		}
		if err = rows.Err(); err != nil {
			logger.Error("GetTrafficLogHandler: Error iterating distinct statuses: %v", err)
		}
	}
	var totalRecords int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM http_traffic_log WHERE %s", finalWhereClause)
	err = database.DB.QueryRow(countQuery, queryArgs...).Scan(&totalRecords)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Error counting traffic logs for target %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Error counting traffic logs"})
		return
	}

	contentTypeQuery := fmt.Sprintf("SELECT DISTINCT response_content_type FROM http_traffic_log WHERE %s ORDER BY response_content_type ASC", finalDistinctWhereClause)
	rows, err = database.DB.Query(contentTypeQuery, distinctQueryArgs...)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Error fetching distinct content types for target %d: %v", targetID, err)
	} else {
		defer rows.Close()
		var contentType sql.NullString
		for rows.Next() {
			if err := rows.Scan(&contentType); err == nil && contentType.Valid {
				distinctValues["type"] = append(distinctValues["type"], contentType.String)
			}
		}
		if err = rows.Err(); err != nil {
			logger.Error("GetTrafficLogHandler: Error iterating distinct content types: %v", err)
		}
	}

	totalPages := int64(0)
	if totalRecords > 0 { // limit is guaranteed to be > 0 due to earlier checks
		totalPages = (totalRecords + int64(limit) - 1) / int64(limit)
	}

	if page > int(totalPages) && totalPages > 0 {
		page = int(totalPages)
		offset = (page - 1) * limit
	}

	allowedSortColumns := map[string]string{
		"id":                    "id",
		"timestamp":             "timestamp",
		"request_method":        "request_method",
		"request_url":           "request_url",
		"response_status_code":  "response_status_code",
		"response_content_type": "response_content_type",
		"response_body_size":    "response_body_size",
		"duration_ms":           "duration_ms",
	}

	dbSortColumn := "timestamp"
	if col, ok := allowedSortColumns[sortByParam]; ok {
		dbSortColumn = col
	}

	dbSortOrder := "DESC"
	if sortOrderParam == "ASC" || sortOrderParam == "DESC" {
		dbSortOrder = sortOrderParam
	}

	orderByClause := fmt.Sprintf("ORDER BY %s %s, id %s", dbSortColumn, dbSortOrder, dbSortOrder)

	finalQueryString := fmt.Sprintf(`SELECT id, timestamp, request_method, request_url, response_status_code,
	                 response_content_type, response_body_size, duration_ms, is_favorite
	          FROM http_traffic_log
	          WHERE %s %s LIMIT ? OFFSET ?`, finalWhereClause, orderByClause)

	finalQueryArgs := append(queryArgs, limit, offset)

	rows, err = database.DB.Query(finalQueryString, finalQueryArgs...)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Error querying traffic logs for target %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Error retrieving traffic logs"})
		return
	}
	defer rows.Close()

	logs := []models.HTTPTrafficLog{}
	for rows.Next() {
		var t models.HTTPTrafficLog
		var statusCode sql.NullInt64
		var contentType sql.NullString
		var bodySize sql.NullInt64
		var duration sql.NullInt64
		var timestampStr string
		var isFavorite sql.NullBool

		if err := rows.Scan(&t.ID, &timestampStr, &t.RequestMethod, &t.RequestURL, &statusCode, &contentType, &bodySize, &duration, &isFavorite); err != nil {
			logger.Error("GetTrafficLogHandler: Error scanning traffic log row: %v", err)
			continue
		}
		parsedTime, tsErr := time.Parse(time.RFC3339, timestampStr)
		if tsErr != nil {
			logger.Info("GetTrafficLogHandler: Could not parse timestamp string '%s' for log ID %d: %v. Using zero time.", timestampStr, t.ID, tsErr)
		}
		t.Timestamp = parsedTime
		if statusCode.Valid {
			t.ResponseStatusCode = int(statusCode.Int64)
		}
		if contentType.Valid {
			t.ResponseContentType = contentType.String
		}
		if bodySize.Valid {
			t.ResponseBodySize = bodySize.Int64
		}
		if duration.Valid {
			t.DurationMs = duration.Int64
		}
		if isFavorite.Valid {
			t.IsFavorite = isFavorite.Bool
		}
		t.TargetID = &targetID
		logs = append(logs, t)
	}
	if err = rows.Err(); err != nil {
		logger.Error("GetTrafficLogHandler: Error iterating traffic log rows: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Error processing traffic log results"})
		return
	}

	response := struct {
		Logs           []models.HTTPTrafficLog `json:"logs"`
		Page           int                     `json:"page"`
		Limit          int                     `json:"limit"`
		TotalRecords   int64                   `json:"total_records"`
		TotalPages     int64                   `json:"total_pages"`
		DistinctValues map[string][]string     `json:"distinct_values"`
	}{
		Logs:           logs,
		Page:           page,
		Limit:          limit,
		TotalRecords:   totalRecords,
		TotalPages:     totalPages,
		DistinctValues: distinctValues,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("GetTrafficLogHandler: Error encoding response for target %d: %v", targetID, err)
	}
	logger.Info("GetTrafficLogHandler: Served %d traffic logs for target %d, page %d", len(logs), targetID, page)
}

// LogEntryDetailResponse is a wrapper for HTTPTrafficLog to include navigation IDs.
type LogEntryDetailResponse struct {
	models.HTTPTrafficLog
	PrevLogID *int64 `json:"prev_log_id,omitempty"`
	NextLogID *int64 `json:"next_log_id,omitempty"`
}

// getTrafficLogEntryDetail fetches full details for a single traffic log entry,
// including IDs for previous/next entries based on current filters.
func getTrafficLogEntryDetail(w http.ResponseWriter, r *http.Request, logID int64) {
	var logEntry models.HTTPTrafficLog
	var targetID sql.NullInt64
	var clientIP sql.NullString
	var serverIP sql.NullString
	var timestampStr string

	query := `SELECT id, target_id, timestamp, request_method, request_url, request_http_version, 
	                 request_headers, request_body, response_status_code, response_reason_phrase, 
					 response_http_version, response_headers, response_body, response_content_type, 
					 response_body_size, duration_ms, client_ip, server_ip, is_https, is_page_candidate, notes,
					 is_favorite
			  FROM http_traffic_log WHERE id = ?`

	err := database.DB.QueryRow(query, logID).Scan(
		&logEntry.ID, &targetID, &timestampStr, &logEntry.RequestMethod, &logEntry.RequestURL,
		&logEntry.RequestHTTPVersion, &logEntry.RequestHeaders, &logEntry.RequestBody,
		&logEntry.ResponseStatusCode, &logEntry.ResponseReasonPhrase, &logEntry.ResponseHTTPVersion,
		&logEntry.ResponseHeaders, &logEntry.ResponseBody, &logEntry.ResponseContentType, &logEntry.ResponseBodySize,
		&logEntry.DurationMs, &clientIP, &serverIP,
		&logEntry.IsHTTPS, &logEntry.IsPageCandidate, &logEntry.Notes, &logEntry.IsFavorite,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			logger.Error("getTrafficLogEntryDetail: Log entry with ID %d not found", logID)
			http.Error(w, "Log entry not found", http.StatusNotFound)
		} else {
			logger.Error("getTrafficLogEntryDetail: Error querying log entry ID %d: %v", logID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	parsedTimestamp, tsErr := time.Parse(time.RFC3339, timestampStr)
	if tsErr != nil {
		logger.Info("getTrafficLogEntryDetail: Could not parse timestamp string '%s' for log ID %d: %v. Using zero time.", timestampStr, logID, tsErr)
	}
	logEntry.Timestamp = parsedTimestamp

	if targetID.Valid {
		logEntry.TargetID = &targetID.Int64
	}
	if clientIP.Valid {
		logEntry.ClientIP = clientIP.String
	}
	if serverIP.Valid {
		logEntry.ServerIP = serverIP.String
	}

	responsePayload := LogEntryDetailResponse{
		HTTPTrafficLog: logEntry,
	}

	if logEntry.TargetID != nil && *logEntry.TargetID > 0 {
		queryParams := r.URL.Query()
		favoritesOnlyStr := queryParams.Get("favorites_only")
		filterMethod := strings.ToUpper(queryParams.Get("method"))
		filterStatus := queryParams.Get("status")
		filterContentType := queryParams.Get("type")
		filterSearchText := queryParams.Get("search")

		filterClauses := []string{"target_id = ?"}
		filterArgs := []interface{}{*logEntry.TargetID}

		if favOnly, errFav := strconv.ParseBool(favoritesOnlyStr); errFav == nil && favOnly {
			filterClauses = append(filterClauses, "is_favorite = TRUE")
		}
		if filterMethod != "" {
			filterClauses = append(filterClauses, "UPPER(request_method) = ?")
			filterArgs = append(filterArgs, filterMethod)
		}
		if filterStatus != "" {
			if statusCode, errStatus := strconv.ParseInt(filterStatus, 10, 64); errStatus == nil {
				filterClauses = append(filterClauses, "response_status_code = ?")
				filterArgs = append(filterArgs, statusCode)
			}
		}
		if filterContentType != "" {
			filterClauses = append(filterClauses, "LOWER(response_content_type) LIKE LOWER(?)")
			filterArgs = append(filterArgs, "%"+filterContentType+"%")
		}
		if filterSearchText != "" {
			searchClause := `(LOWER(request_url) LIKE LOWER(?) OR UPPER(request_method) LIKE UPPER(?) OR LOWER(response_content_type) LIKE LOWER(?) OR CAST(response_status_code AS TEXT) LIKE ?)`
			filterClauses = append(filterClauses, searchClause)
			searchPattern := "%" + filterSearchText + "%"
			filterArgs = append(filterArgs, searchPattern, searchPattern, searchPattern, searchPattern)
		}

		filterCondition := strings.Join(filterClauses, " AND ")
		var prevID, nextID sql.NullInt64

		prevQuerySQL := fmt.Sprintf(`SELECT id FROM http_traffic_log 
		                             WHERE %s AND ((timestamp = ? AND id < ?) OR timestamp < ?) 
									 ORDER BY timestamp DESC, id DESC LIMIT 1`, filterCondition)
		prevArgs := append(filterArgs, logEntry.Timestamp, logEntry.ID, logEntry.Timestamp)

		errPrev := database.DB.QueryRow(prevQuerySQL, prevArgs...).Scan(&prevID)
		if errPrev == nil && prevID.Valid {
			responsePayload.PrevLogID = &prevID.Int64
		} else if errPrev != nil && errPrev != sql.ErrNoRows {
			logger.Error("getTrafficLogEntryDetail: Error querying previous (filtered) log ID for log %d: %v", logID, errPrev)
		}

		nextQuerySQL := fmt.Sprintf(`SELECT id FROM http_traffic_log 
		                             WHERE %s AND ((timestamp = ? AND id > ?) OR timestamp > ?) 
									 ORDER BY timestamp ASC, id ASC LIMIT 1`, filterCondition)
		nextArgs := append(filterArgs, logEntry.Timestamp, logEntry.ID, logEntry.Timestamp)

		errNext := database.DB.QueryRow(nextQuerySQL, nextArgs...).Scan(&nextID)
		if errNext == nil && nextID.Valid {
			responsePayload.NextLogID = &nextID.Int64
		} else if errNext != nil && errNext != sql.ErrNoRows {
			logger.Error("getTrafficLogEntryDetail: Error querying next (filtered) log ID for log %d: %v", logID, errNext)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(responsePayload); err != nil {
		logger.Error("getTrafficLogEntryDetail: Error encoding response for log ID %d: %v", logID, err)
	}
	logger.Info("Successfully served details for log entry ID %d", logID)
}

// updateTrafficLogEntryNotes updates the notes for a specific traffic log entry.
func updateTrafficLogEntryNotes(w http.ResponseWriter, r *http.Request, logID int64) {
	var req struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("updateTrafficLogEntryNotes: Error decoding request body for log ID %d: %v", logID, err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	stmt, err := database.DB.Prepare("UPDATE http_traffic_log SET notes = ? WHERE id = ?")
	if err != nil {
		logger.Error("updateTrafficLogEntryNotes: Error preparing update statement for log ID %d: %v", logID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(req.Notes, logID)
	if err != nil {
		logger.Error("updateTrafficLogEntryNotes: Error executing update for log ID %d: %v", logID, err)
		http.Error(w, "Internal server error during update", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("updateTrafficLogEntryNotes: Log entry with ID %d not found for notes update", logID)
		http.Error(w, fmt.Sprintf("Log entry with ID %d not found", logID), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Notes updated successfully."})
	logger.Info("Successfully updated notes for log entry ID %d", logID)
}

// setTrafficLogEntryFavoriteStatus updates the favorite status for a specific traffic log entry.
func setTrafficLogEntryFavoriteStatus(w http.ResponseWriter, r *http.Request, logID int64) {
	var req struct {
		IsFavorite bool `json:"is_favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("setTrafficLogEntryFavoriteStatus: Error decoding request body for log ID %d: %v", logID, err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	stmt, err := database.DB.Prepare("UPDATE http_traffic_log SET is_favorite = ? WHERE id = ?")
	if err != nil {
		logger.Error("setTrafficLogEntryFavoriteStatus: Error preparing update statement for log ID %d: %v", logID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(req.IsFavorite, logID)
	if err != nil {
		logger.Error("setTrafficLogEntryFavoriteStatus: Error executing update for log ID %d: %v", logID, err)
		http.Error(w, "Internal server error during update", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("setTrafficLogEntryFavoriteStatus: Log entry with ID %d not found for favorite update", logID)
		http.Error(w, fmt.Sprintf("Log entry with ID %d not found", logID), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Favorite status updated successfully.", "is_favorite": req.IsFavorite})
	logger.Info("Successfully updated favorite status for log entry ID %d to %v", logID, req.IsFavorite)
}