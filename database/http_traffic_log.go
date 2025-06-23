package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"sort"
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

	// Select from http_traffic_log and LEFT JOIN pages
	selectBase := "SELECT htl.id, htl.target_id, htl.timestamp, htl.request_method, htl.request_url, htl.request_full_url_with_fragment, htl.response_status_code, htl.response_content_type, htl.response_body_size, htl.duration_ms, htl.is_favorite, htl.log_source, htl.page_sitemap_id, p.name AS page_sitemap_name"
	fromAndJoinBase := "FROM http_traffic_log htl LEFT JOIN pages p ON htl.page_sitemap_id = p.id"
	whereClauses := []string{"htl.target_id = ?"} // Explicitly use htl.target_id
	args := []interface{}{filters.TargetID}
	countArgs := []interface{}{filters.TargetID} // Initialize countArgs here

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
	logger.Debug("GetHTTPTrafficLogEntries: FilterDomain received: '%s'", filters.FilterDomain)
	// Filter by domain (hostname part of the URL)
	if filters.FilterDomain != "" {
		// This pattern attempts to match the exact domain name as the hostname.
		// It covers:
		// - http://domain.com
		// - https://domain.com
		// - http://domain.com/path
		// - https://domain.com?query
		// - http://domain.com:port
		// - https://domain.com:port
		whereClauses = append(whereClauses, `(
            request_url LIKE 'http://' || ? || '%' OR
            request_url LIKE 'https://' || ? || '%'
        )`)
		args = append(args, filters.FilterDomain, filters.FilterDomain)
		countArgs = append(countArgs, filters.FilterDomain, filters.FilterDomain)
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

	countQuery := fmt.Sprintf("SELECT COUNT(htl.id) %s %s", fromAndJoinBase, finalWhereClause)
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
	// Add page_sitemap_name to valid sort columns, mapping to p.name
	validSortColumns := map[string]string{
		"id": "htl.id", "timestamp": "htl.timestamp", "request_method": "htl.request_method", // Prefix with htl
		"request_url": "htl.request_url", "response_status_code": "htl.response_status_code",
		"response_content_type": "htl.response_content_type", "response_body_size": "htl.response_body_size",
		"page_sitemap_id": "htl.page_sitemap_id", "page_sitemap_name": "p.name", // p.name is already specific
		"duration_ms": "htl.duration_ms",
	}
	if col, ok := validSortColumns[filters.SortBy]; ok {
		orderBy = col
	} else {
		orderBy = "htl.timestamp" // Default to htl.timestamp if sortBy is invalid
	}

	// Define query and queryArgs before they are potentially used in logging
	query := fmt.Sprintf("%s %s %s ORDER BY %s %s, htl.id %s LIMIT ? OFFSET ?",
		selectBase, fromAndJoinBase, finalWhereClause, orderBy, sortOrder, sortOrder)
	queryArgs := append(args, filters.Limit, (filters.Page-1)*filters.Limit)

	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		logger.Error("GetHTTPTrafficLogEntries: Error querying records: %v. Query: %s. Args: %v", err, query, queryArgs)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var u models.HTTPTrafficLog
		var timestampStr string
		if err := rows.Scan(&u.ID, &u.TargetID, &timestampStr, &u.RequestMethod, &u.RequestURL, &u.RequestFullURLWithFragment, &u.ResponseStatusCode, &u.ResponseContentType, &u.ResponseBodySize, &u.DurationMs, &u.IsFavorite, &u.LogSource, &u.PageSitemapID, &u.PageSitemapName); err != nil {
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
	query := `SELECT htl.id, htl.target_id, htl.timestamp, htl.request_method, htl.request_url, htl.request_full_url_with_fragment, 
	                 htl.request_http_version, htl.request_headers, htl.request_body, 
	                 htl.response_status_code, htl.response_content_type, htl.response_body_size, htl.response_http_version, 
	                 htl.response_headers, htl.response_body, htl.duration_ms, htl.is_favorite, htl.notes, 
	                 htl.log_source, htl.page_sitemap_id, p.name AS page_sitemap_name
	          FROM http_traffic_log htl LEFT JOIN pages p ON htl.page_sitemap_id = p.id WHERE htl.id = ?`
	var timestampStr string
	err := DB.QueryRow(query, id).Scan(
		&log.ID, &log.TargetID, &timestampStr, &log.RequestMethod, &log.RequestURL,
		&log.RequestFullURLWithFragment, &log.RequestHTTPVersion,
		&log.RequestHeaders, // This will scan into sql.NullString if model is updated
		&log.RequestBody,
		&log.ResponseStatusCode, &log.ResponseContentType, &log.ResponseBodySize, &log.ResponseHTTPVersion, &log.ResponseHeaders, &log.ResponseBody,
		&log.DurationMs, &log.IsFavorite, &log.Notes, &log.LogSource, &log.PageSitemapID,
		&log.PageSitemapName) // Scan the page name
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("GetHTTPTrafficLogEntryByID: No log entry found for ID %d", id)
			return log, fmt.Errorf("HTTP traffic log entry with ID %d not found", id)
		}
		// Use a more distinct message for this critical scan error
		logger.Error("GetHTTPTrafficLogEntryByID: CRITICAL - Error scanning main log entry data for ID %d: %v", id, err)
		return log, err
	}

	// Log the state of RequestFullURLWithFragment immediately after scan
	logger.Debug("GetHTTPTrafficLogEntryByID: Main log entry scan for ID %d SUCCESSFUL. Proceeding to associated findings.", id)
	logger.Debug("GetHTTPTrafficLogEntryByID - Scanned for ID %d: log.RequestFullURLWithFragment.Valid = %t, log.RequestFullURLWithFragment.String = '%s'", id, log.RequestFullURLWithFragment.Valid, log.RequestFullURLWithFragment.String)

	logger.Debug("GetHTTPTrafficLogEntryByID: Attempting to fetch associated findings for log ID %d", id)
	// Fetch associated findings
	rowsFindings, errFindings := DB.Query("SELECT id, title FROM target_findings WHERE http_traffic_log_id = ?", id)
	if errFindings != nil {
		// Log error but don't fail the whole log retrieval
		logger.Error("GetHTTPTrafficLogEntryByID: Error fetching associated findings for log ID %d: %v", id, errFindings)
		log.AssociatedFindings = []models.FindingLink{} // Ensure it's an empty slice on error
	} else {
		defer rowsFindings.Close()
		var findings []models.FindingLink
		logger.Debug("GetHTTPTrafficLogEntryByID: Query for associated findings for log ID %d successful. Processing rows...", id)
		rowCount := 0
		for rowsFindings.Next() {
			rowCount++
			var f models.FindingLink
			if errScan := rowsFindings.Scan(&f.ID, &f.Title); errScan != nil {
				logger.Error("GetHTTPTrafficLogEntryByID: Error scanning associated finding for log ID %d (row %d): %v", id, rowCount, errScan)
				continue // Skip this finding on scan error
			}
			findings = append(findings, f)
			logger.Debug("GetHTTPTrafficLogEntryByID: Scanned finding: ID=%d, Title='%s'", f.ID, f.Title)
		}
		if errRows := rowsFindings.Err(); errRows != nil {
			logger.Error("GetHTTPTrafficLogEntryByID: Error after iterating finding rows for log ID %d: %v", id, errRows)
		}
		logger.Debug("GetHTTPTrafficLogEntryByID: Total associated findings scanned for log ID %d: %d. Assigning to log.AssociatedFindings.", id, len(findings))
		log.AssociatedFindings = findings
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
		target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body, request_full_url_with_fragment,
		response_status_code, response_reason_phrase, response_http_version, response_headers, response_body, response_content_type,
		response_body_size, duration_ms, client_ip, is_https, is_page_candidate, notes, source_modifier_task_id,
		log_source, page_sitemap_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, // Added placeholders
		logEntry.TargetID, logEntry.Timestamp, logEntry.RequestMethod, logEntry.RequestURL,
		logEntry.RequestHTTPVersion, logEntry.RequestHeaders, logEntry.RequestBody,
		logEntry.RequestFullURLWithFragment, // Ensure this is passed if applicable
		logEntry.ResponseStatusCode, logEntry.ResponseReasonPhrase, logEntry.ResponseHTTPVersion,
		logEntry.ResponseHeaders, logEntry.ResponseBody, logEntry.ResponseContentType,
		logEntry.ResponseBodySize, logEntry.DurationMs, logEntry.ClientIP, logEntry.IsHTTPS,
		logEntry.IsPageCandidate, logEntry.Notes, logEntry.SourceModifierTaskID, // Existing fields
		models.NullString("Modifier"), sql.NullInt64{Valid: false}, // Set log_source to "Modifier", page_sitemap_id to NULL
	)
	// Note: is_favorite defaults to FALSE in schema, not explicitly set here.
	if err != nil {
		logger.Error("DB log error for modified request (%s %s): %v", logEntry.RequestMethod.String, logEntry.RequestURL.String, err)
		return 0, err
	}
	return result.LastInsertId()
}

// GetDistinctDomainsFromLogs retrieves a list of distinct hostnames (domains)
// from the http_traffic_log for a given target.
func GetDistinctDomainsFromLogs(targetID int64) ([]string, error) {
	logger.Debug("GetDistinctDomainsFromLogs: Called for targetID: %d", targetID)
	if DB == nil {
		logger.Error("GetDistinctDomainsFromLogs: Database connection is not initialized.")
		return nil, fmt.Errorf("database connection is not initialized")
	}

	query := `
		SELECT DISTINCT request_url
		FROM http_traffic_log
		WHERE target_id = ? AND request_url IS NOT NULL AND request_url != ''
	`
	rows, err := DB.Query(query, targetID)
	if err != nil {
		logger.Error("GetDistinctDomainsFromLogs: Error querying distinct URLs for target %d: %v", targetID, err)
		return nil, fmt.Errorf("querying distinct URLs failed: %w", err)
	}
	defer rows.Close()

	domainMap := make(map[string]struct{}) // Use a map to ensure uniqueness
	processedURLsCount := 0
	parsedHostnamesCount := 0

	for rows.Next() {
		processedURLsCount++
		var rawURL string
		if err := rows.Scan(&rawURL); err != nil {
			logger.Error("GetDistinctDomainsFromLogs: Error scanning raw URL: %v", err)
			continue
		}
		logger.Debug("GetDistinctDomainsFromLogs: Processing raw URL: '%s'", rawURL)
		if u, err := url.Parse(rawURL); err == nil && u.Hostname() != "" {
			parsedHostnamesCount++
			domainMap[u.Hostname()] = struct{}{}
			logger.Debug("GetDistinctDomainsFromLogs: Extracted hostname: '%s'", u.Hostname())
		} else {
			logger.Debug("GetDistinctDomainsFromLogs: Failed to parse URL '%s' or extract hostname. Error: %v, Hostname: '%s'", rawURL, err, u.Hostname())
		}
	}

	domains := make([]string, 0, len(domainMap))
	for domain := range domainMap {
		domains = append(domains, domain)
	}
	sort.Strings(domains) // Sort the domains alphabetically

	logger.Debug("GetDistinctDomainsFromLogs: Finished processing for targetID %d. Total distinct URLs processed: %d. Total hostnames extracted: %d. Final distinct domains: %d.", targetID, processedURLsCount, parsedHostnamesCount, len(domains))
	return domains, nil
}
