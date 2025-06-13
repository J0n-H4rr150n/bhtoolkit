package database

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

// GetHTTPTrafficLogEntries retrieves paginated and filtered HTTP traffic log entries.
// It's a simplified version of what's in GetTrafficLogHandler, focused on fetching.
func GetHTTPTrafficLogEntries(filters models.ProxyLogFilters) ([]models.HTTPTrafficLog, int64, error) {
	var logs []models.HTTPTrafficLog
	var totalRecords int64

	if filters.TargetID == 0 {
		return nil, 0, fmt.Errorf("TargetID is required to fetch HTTP traffic logs")
	}

	baseQuery := "FROM http_traffic_log"
	whereClauses := []string{"target_id = ?"}
	args := []interface{}{filters.TargetID}

	if filters.FilterFavoritesOnly {
		whereClauses = append(whereClauses, "is_favorite = TRUE")
	}
	if filters.FilterMethod != "" {
		whereClauses = append(whereClauses, "UPPER(request_method) = ?")
		args = append(args, strings.ToUpper(filters.FilterMethod))
	}
	if filters.FilterStatus != "" {
		statusCode, err := strconv.ParseInt(filters.FilterStatus, 10, 64)
		if err == nil {
			whereClauses = append(whereClauses, "response_status_code = ?")
			args = append(args, statusCode)
		}
	}
	if filters.FilterContentType != "" {
		whereClauses = append(whereClauses, "LOWER(response_content_type) LIKE LOWER(?)")
		args = append(args, "%"+filters.FilterContentType+"%")
	}
	if filters.FilterSearchText != "" {
		// Expand search to include headers and bodies.
		// SQLite's LIKE operator works on BLOBs containing text.
		searchClause := `(
			LOWER(request_url) LIKE LOWER(?) OR
			UPPER(request_method) LIKE UPPER(?) OR
			LOWER(response_content_type) LIKE LOWER(?) OR
			CAST(response_status_code AS TEXT) LIKE ? OR
			LOWER(request_headers) LIKE LOWER(?) OR -- Search request headers (JSON string)
			request_body LIKE ? OR -- Search request body (BLOB, assumes text content)
			LOWER(response_headers) LIKE LOWER(?) OR -- Search response headers (JSON string)
			response_body LIKE ? -- Search response body (BLOB, assumes text content)
		)`
		whereClauses = append(whereClauses, searchClause)
		searchPattern := "%" + filters.FilterSearchText + "%"
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	finalWhereClause := ""
	if len(whereClauses) > 0 {
		finalWhereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) %s %s", baseQuery, finalWhereClause)
	err := DB.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		logger.Error("GetHTTPTrafficLogEntries: Error counting records: %v", err)
		return nil, 0, err
	}

	if totalRecords == 0 {
		return logs, 0, nil
	}

	sortOrder := "DESC"
	if strings.ToUpper(filters.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	orderBy := "timestamp" // Default sort
	validSortColumns := map[string]string{
		"id": "id", "timestamp": "timestamp", "request_method": "request_method",
		"request_url": "request_url", "response_status_code": "response_status_code",
		"response_content_type": "response_content_type", "response_body_size": "response_body_size",
		"duration_ms": "duration_ms",
	}
	if col, ok := validSortColumns[filters.SortBy]; ok {
		orderBy = col
	}

	query := fmt.Sprintf("SELECT id, target_id, timestamp, request_method, request_url, response_status_code, response_content_type, response_body_size, duration_ms, is_favorite %s %s ORDER BY %s %s, id %s LIMIT ? OFFSET ?",
		baseQuery, finalWhereClause, orderBy, sortOrder, sortOrder)
	queryArgs := append(args, filters.Limit, (filters.Page-1)*filters.Limit)

	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		logger.Error("GetHTTPTrafficLogEntries: Error querying records: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var u models.HTTPTrafficLog
		var timestampStr string
		// Note: Scanning only a subset of fields needed by AnalyzeTargetForParameterizedURLsHandler
		// Adjust if more fields are needed by other callers of this function.
		if err := rows.Scan(&u.ID, &u.TargetID, &timestampStr, &u.RequestMethod, &u.RequestURL, &u.ResponseStatusCode, &u.ResponseContentType, &u.ResponseBodySize, &u.DurationMs, &u.IsFavorite); err != nil {
			// If RequestMethod or RequestURL are now sql.NullString, this Scan will fail if they are not sql.NullString in the struct.
			// Assuming the struct models.HTTPTrafficLog is updated, this Scan should be fine.
			// The fields u.RequestMethod and u.RequestURL are now sql.NullString.
			logger.Error("GetHTTPTrafficLogEntries: Error scanning row for log ID %d: %v", u.ID, err)
			// Decide whether to return partial results or error out
			continue
		}
		parsedTime, _ := time.Parse(time.RFC3339, timestampStr) // Error handling for time parsing can be added
		u.Timestamp = parsedTime
		logs = append(logs, u)
	}
	return logs, totalRecords, rows.Err()
}

// GetHTTPTrafficLogEntryByID retrieves a single HTTP traffic log entry by its ID.
func GetHTTPTrafficLogEntryByID(id int64) (models.HTTPTrafficLog, error) {
	var log models.HTTPTrafficLog
	// Select all fields that might be needed for the modifier's base request
	query := `SELECT id, target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
	                 response_status_code, response_content_type, response_body_size, response_http_version, response_headers, response_body,
	                 duration_ms, is_favorite, notes
	          FROM http_traffic_log WHERE id = ?`
	var timestampStr string
	err := DB.QueryRow(query, id).Scan(
		&log.ID, &log.TargetID, &timestampStr, &log.RequestMethod, &log.RequestURL,
		&log.RequestHTTPVersion, // This is string
		&log.RequestHeaders,     // This will scan into sql.NullString if model is updated
		&log.RequestBody,
		&log.ResponseStatusCode, &log.ResponseContentType, &log.ResponseBodySize, &log.ResponseHTTPVersion, &log.ResponseHeaders, &log.ResponseBody,
		&log.DurationMs, &log.IsFavorite, &log.Notes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return log, fmt.Errorf("HTTP traffic log entry with ID %d not found", id)
		}
		logger.Error("GetHTTPTrafficLogEntryByID: Error scanning log ID %d: %v", id, err)
		return log, err
	}
	parsedTime, _ := time.Parse(time.RFC3339, timestampStr)
	log.Timestamp = parsedTime
	return log, nil
}

// LogExecutedModifierRequest saves an HTTPTrafficLog entry generated from the modifier.
func LogExecutedModifierRequest(logEntry *models.HTTPTrafficLog) (int64, error) {
	if DB == nil {
		logger.Error("LogExecutedModifierRequest: Database is not initialized.")
		return 0, fmt.Errorf("database not initialized")
	}
	result, err := DB.Exec(`INSERT INTO http_traffic_log (
		target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
		response_status_code, response_reason_phrase, response_http_version, response_headers, response_body,
		response_content_type, response_body_size, duration_ms, client_ip, is_https, is_page_candidate, notes,
		source_modifier_task_id 
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		logEntry.TargetID, logEntry.Timestamp, logEntry.RequestMethod, logEntry.RequestURL,
		logEntry.RequestHTTPVersion, logEntry.RequestHeaders, logEntry.RequestBody,
		logEntry.ResponseStatusCode, logEntry.ResponseReasonPhrase, logEntry.ResponseHTTPVersion,
		logEntry.ResponseHeaders, logEntry.ResponseBody, logEntry.ResponseContentType,
		logEntry.ResponseBodySize, logEntry.DurationMs, logEntry.ClientIP, logEntry.IsHTTPS,
		logEntry.IsPageCandidate, logEntry.Notes, logEntry.SourceModifierTaskID,
	)
	// Note: is_favorite defaults to FALSE in schema, not explicitly set here.
	if err != nil {
		logger.Error("DB log error for modified request (%s %s): %v", logEntry.RequestMethod.String, logEntry.RequestURL.String, err)
		return 0, err
	}
	return result.LastInsertId()
}
