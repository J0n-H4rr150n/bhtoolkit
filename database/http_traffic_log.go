package database

import (
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
		searchClause := `(LOWER(request_url) LIKE LOWER(?) OR UPPER(request_method) LIKE UPPER(?) OR LOWER(response_content_type) LIKE LOWER(?) OR CAST(response_status_code AS TEXT) LIKE ?)`
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
			logger.Error("GetHTTPTrafficLogEntries: Error scanning row: %v", err)
			// Decide whether to return partial results or error out
			continue
		}
		parsedTime, _ := time.Parse(time.RFC3339, timestampStr) // Error handling for time parsing can be added
		u.Timestamp = parsedTime
		logs = append(logs, u)
	}
	return logs, totalRecords, rows.Err()
}
