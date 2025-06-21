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
	filterTagIDsStr := r.URL.Query().Get("filter_tag_ids") // New: Filter by tag IDs

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

	whereClauses := []string{"htl.target_id = ?"} // Qualified target_id
	queryArgs := []interface{}{targetID}

	var tagIDsFilter []int64
	if filterTagIDsStr != "" {
		tagIDParts := strings.Split(filterTagIDsStr, ",")
		for _, part := range tagIDParts {
			trimmedPart := strings.TrimSpace(part)
			if id, errConv := strconv.ParseInt(trimmedPart, 10, 64); errConv == nil && id > 0 {
				tagIDsFilter = append(tagIDsFilter, id)
			}
		}
	}

	distinctWhereClauses := []string{"target_id = ?"}
	distinctQueryArgs := []interface{}{targetID}

	if favOnly, err := strconv.ParseBool(favoritesOnlyStr); err == nil && favOnly {
		whereClauses = append(whereClauses, "htl.is_favorite = TRUE") // Alias added
		distinctWhereClauses = append(distinctWhereClauses, "is_favorite = TRUE")
	}

	if filterMethod != "" {
		whereClauses = append(whereClauses, "UPPER(htl.request_method) = ?") // Alias added
		queryArgs = append(queryArgs, filterMethod)
		// distinctWhereClauses for method is not explicitly added here, it's part of finalDistinctWhereClause construction
	}

	if filterStatus != "" {
		statusCode, err := strconv.ParseInt(filterStatus, 10, 64)
		if err == nil {
			whereClauses = append(whereClauses, "htl.response_status_code = ?") // Alias added
			queryArgs = append(queryArgs, statusCode)
		} else {
			logger.Info("GetTrafficLogHandler: Invalid status code filter '%s': %v. Ignoring filter.", filterStatus, err)
		}
	}

	if filterContentType != "" {
		whereClauses = append(whereClauses, "LOWER(htl.response_content_type) LIKE LOWER(?)") // Alias added
		queryArgs = append(queryArgs, "%"+filterContentType+"%")
	}

	if filterSearchText != "" {
		unaliasedSearchClause := `(LOWER(request_url) LIKE LOWER(?) OR UPPER(request_method) LIKE UPPER(?) OR LOWER(response_content_type) LIKE LOWER(?) OR CAST(response_status_code AS TEXT) LIKE ?)`
		aliasedSearchClause := `(LOWER(htl.request_url) LIKE LOWER(?) OR UPPER(htl.request_method) LIKE UPPER(?) OR LOWER(htl.response_content_type) LIKE LOWER(?) OR CAST(htl.response_status_code AS TEXT) LIKE ?)`
		whereClauses = append(whereClauses, aliasedSearchClause)
		distinctWhereClauses = append(distinctWhereClauses, unaliasedSearchClause)
		searchPattern := "%" + filterSearchText + "%"
		queryArgs = append(queryArgs, searchPattern, searchPattern, searchPattern, searchPattern)
		distinctQueryArgs = append(distinctQueryArgs, searchPattern, searchPattern, searchPattern, searchPattern)
	}

	if len(tagIDsFilter) > 0 {
		placeholders := make([]string, len(tagIDsFilter))
		tagIDArgs := make([]interface{}, len(tagIDsFilter))
		for i, id := range tagIDsFilter {
			placeholders[i] = "?"
			tagIDArgs[i] = id
		}
		tagFilterClause := fmt.Sprintf("htl.id IN (SELECT DISTINCT item_id FROM tag_associations WHERE item_type = 'httplog' AND tag_id IN (%s))", strings.Join(placeholders, ","))
		whereClauses = append(whereClauses, tagFilterClause)
		queryArgs = append(queryArgs, tagIDArgs...)

		// For distinct values, we need to ensure the logs considered for distinct methods/statuses etc. also match the tag filter.
		// The distinctWhereClauses and distinctQueryArgs are used for fetching distinct methods, statuses, types.
		// If we want these dropdowns to be reactive to the tag filter, we add the tag filter here too.
		distinctWhereClauses = append(distinctWhereClauses, fmt.Sprintf("id IN (SELECT DISTINCT item_id FROM tag_associations WHERE item_type = 'httplog' AND tag_id IN (%s))", strings.Join(placeholders, ",")))
		distinctQueryArgs = append(distinctQueryArgs, tagIDArgs...)
	}

	finalWhereClause := strings.Join(whereClauses, " AND ")
	finalDistinctWhereClause := strings.Join(distinctWhereClauses, " AND ")
	distinctValues := make(map[string]interface{}) // Changed to interface{} to support different value types

	methodQuery := fmt.Sprintf("SELECT DISTINCT request_method FROM http_traffic_log WHERE %s ORDER BY request_method ASC", finalDistinctWhereClause)
	rows, err := database.DB.Query(methodQuery, distinctQueryArgs...)
	if err != nil {
		logger.Error("GetTrafficLogHandler: Error fetching distinct methods for target %d: %v", targetID, err)
	} else {
		defer rows.Close()
		var method string
		for rows.Next() {
			if err := rows.Scan(&method); err == nil && strings.TrimSpace(method) != "" {
				// Ensure distinctValues["method"] is initialized as []string if it doesn't exist or is not the correct type
				currentMethods, ok := distinctValues["method"].([]string)
				if !ok {
					currentMethods = []string{}
				}
				distinctValues["method"] = append(currentMethods, method)
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
				currentStatuses, ok := distinctValues["status"].([]string)
				if !ok {
					currentStatuses = []string{}
				}
				distinctValues["status"] = append(currentStatuses, strconv.FormatInt(status.Int64, 10))
			}
		}
		if err = rows.Err(); err != nil {
			logger.Error("GetTrafficLogHandler: Error iterating distinct statuses: %v", err)
		}
	}
	var totalRecords int64
	countQuery := fmt.Sprintf("SELECT COUNT(htl.id) FROM http_traffic_log htl WHERE %s", finalWhereClause)
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
			if err := rows.Scan(&contentType); err == nil && contentType.Valid && strings.TrimSpace(contentType.String) != "" {
				currentTypes, ok := distinctValues["type"].([]string)
				if !ok {
					currentTypes = []string{}
				}
				distinctValues["type"] = append(currentTypes, contentType.String)
			}
		}
		if err = rows.Err(); err != nil {
			logger.Error("GetTrafficLogHandler: Error iterating distinct content types: %v", err)
		}
	}

	// Fetch all distinct tags for the CURRENT TARGET, ignoring other filters.
	// This ensures the dropdown always shows all relevant tags for the target,
	// allowing users to add/remove from their filter selection without options disappearing.
	distinctTagQuery := fmt.Sprintf(`
		SELECT DISTINCT t.id, t.name, t.color
		FROM tags t 
		JOIN tag_associations ta ON t.id = ta.tag_id
		JOIN http_traffic_log htl_tags ON ta.item_id = htl_tags.id AND ta.item_type = 'httplog'
		WHERE htl_tags.target_id = ?
		ORDER BY LOWER(t.name) ASC
	`)

	tagRows, errTag := database.DB.Query(distinctTagQuery, targetID) // Use only targetID for this query
	if errTag != nil {
		logger.Error("GetTrafficLogHandler: Error fetching all distinct tags for target %d: %v", targetID, errTag)
	} else {
		var distinctTags []models.Tag
		for tagRows.Next() {
			var tag models.Tag
			if err := tagRows.Scan(&tag.ID, &tag.Name, &tag.Color); err == nil {
				distinctTags = append(distinctTags, tag)
			}
		}
		tagRows.Close()
		if errRows := tagRows.Err(); errRows != nil {
			logger.Error("GetTrafficLogHandler: Error iterating distinct tags: %v", errRows)
		}
		distinctValues["tags"] = distinctTags // Store as []models.Tag
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
		"id":                    "htl.id",
		"timestamp":             "htl.timestamp",
		"request_method":        "htl.request_method",
		"request_url":           "htl.request_url",
		"response_status_code":  "htl.response_status_code",
		"response_content_type": "htl.response_content_type",
		"response_body_size":    "htl.response_body_size",
		"page_sitemap_id":       "htl.page_sitemap_id", // Qualified
		"tags":                  "(SELECT GROUP_CONCAT(t_sort.name ORDER BY t_sort.name ASC) FROM tags t_sort JOIN tag_associations ta_sort ON t_sort.id = ta_sort.tag_id WHERE ta_sort.item_id = htl.id AND ta_sort.item_type = 'httplog')",
		"page_sitemap_name":     "page_sitemap_name", // This is an alias from SELECT
		"duration_ms":           "htl.duration_ms",
	}

	dbSortColumn := "htl.timestamp" // Default sort, aliased
	if col, ok := allowedSortColumns[sortByParam]; ok {
		dbSortColumn = col
	}

	dbSortOrder := "DESC"
	if sortOrderParam == "ASC" || sortOrderParam == "DESC" {
		dbSortOrder = sortOrderParam
	}

	orderByClause := fmt.Sprintf("ORDER BY %s %s, htl.id %s", dbSortColumn, dbSortOrder, dbSortOrder) // Alias id

	finalQueryString := fmt.Sprintf(`SELECT htl.id, htl.timestamp, htl.request_method, htl.request_url, htl.request_full_url_with_fragment,
	                 htl.response_status_code, htl.response_content_type, htl.response_body_size, htl.duration_ms, htl.is_favorite,
	                 htl.log_source, htl.page_sitemap_id, p.name as page_sitemap_name
	          FROM http_traffic_log htl LEFT JOIN pages p ON htl.page_sitemap_id = p.id
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

		// Add &t.RequestFullURLWithFragment, &t.LogSource, &t.PageSitemapID, &t.PageSitemapName to the Scan call
		if err := rows.Scan(&t.ID, &timestampStr, &t.RequestMethod, &t.RequestURL, &t.RequestFullURLWithFragment, &statusCode, &contentType, &bodySize, &duration, &isFavorite, &t.LogSource, &t.PageSitemapID, &t.PageSitemapName); err != nil {
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
		// contentType is already sql.NullString, assign directly
		t.ResponseContentType = contentType

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

	// Fetch tags for all logs retrieved in this page
	if len(logs) > 0 {
		logIDs := make([]int64, len(logs))
		for i, lg := range logs {
			logIDs[i] = lg.ID
		}
		tagsByLogID, errTags := database.GetTagsForMultipleItems(logIDs, "httplog")
		if errTags != nil {
			logger.Error("GetTrafficLogHandler: Error fetching tags for multiple log entries: %v", errTags)
			// Continue without tags if there's an error
		} else {
			for i := range logs {
				logs[i].Tags = tagsByLogID[logs[i].ID] // Assign tags, will be empty slice if no tags
			}
		}
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
		TotalPages     int64                   `json:"total_pages"` // Corrected type
		DistinctValues map[string]interface{}  `json:"distinct_values"`
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
	PrevLogID *int64       `json:"prev_log_id,omitempty"`
	NextLogID *int64       `json:"next_log_id,omitempty"`
	Tags      []models.Tag `json:"tags,omitempty"` // Added to include associated tags
}

// getTrafficLogEntryDetail fetches full details for a single traffic log entry,
// including IDs for previous/next entries based on current filters.
func getTrafficLogEntryDetail(w http.ResponseWriter, r *http.Request, logID int64) {
	var logEntry models.HTTPTrafficLog
	var targetID sql.NullInt64
	var clientIP sql.NullString
	var serverIP sql.NullString // This is the scan target
	var timestampStr string
	// Local sql.NullString for notes, as it's handled separately after scan
	var notes sql.NullString

	query := `SELECT htl.id, htl.target_id, htl.timestamp, htl.request_method, htl.request_url, 
	                 htl.request_http_version, htl.request_headers, htl.request_body, 
					 htl.response_status_code, htl.response_reason_phrase, htl.response_http_version, 
					 htl.response_headers, htl.response_body, htl.response_content_type, 
					 htl.response_body_size, htl.duration_ms, htl.client_ip, htl.server_ip, 
					 htl.is_https, htl.is_page_candidate, htl.notes, htl.is_favorite, 
					 htl.request_full_url_with_fragment, htl.page_sitemap_id, p.name as page_sitemap_name
			  FROM http_traffic_log htl
			  LEFT JOIN pages p ON htl.page_sitemap_id = p.id
			  WHERE htl.id = ?`

	err := database.DB.QueryRow(query, logID).Scan(
		&logEntry.ID, &targetID, &timestampStr, &logEntry.RequestMethod, &logEntry.RequestURL,
		&logEntry.RequestHTTPVersion, &logEntry.RequestHeaders, &logEntry.RequestBody,
		&logEntry.ResponseStatusCode, &logEntry.ResponseReasonPhrase, &logEntry.ResponseHTTPVersion,
		&logEntry.ResponseHeaders, &logEntry.ResponseBody, &logEntry.ResponseContentType, &logEntry.ResponseBodySize,
		&logEntry.DurationMs, &clientIP, &serverIP,
		&logEntry.IsHTTPS, &logEntry.IsPageCandidate, &notes, &logEntry.IsFavorite,
		&logEntry.RequestFullURLWithFragment,
		&logEntry.PageSitemapID, &logEntry.PageSitemapName, // Scan the new page sitemap fields
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
	// Assign the scanned sql.NullString directly
	logEntry.ClientIP = clientIP
	logEntry.ServerIP = serverIP
	logEntry.Notes = notes

	// The following fields were already being scanned directly into logEntry which is correct
	// as they are sql.NullString in the model:
	// logEntry.RequestMethod
	// logEntry.RequestURL
	// logEntry.RequestHTTPVersion
	// logEntry.RequestHeaders
	// logEntry.ResponseReasonPhrase, logEntry.ResponseHTTPVersion, logEntry.ResponseHeaders, logEntry.ResponseContentType
	// logEntry.RequestFullURLWithFragment

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

	// Fetch associated tags
	tags, tagsErr := database.GetTagsForItem(logID, "httplog") // Assuming "httplog" is the item_type for traffic logs
	if tagsErr != nil {
		logger.Error("getTrafficLogEntryDetail: Error fetching tags for log ID %d: %v", logID, tagsErr)
		// Do not fail the entire request if tags can't be fetched, just log it.
		responsePayload.Tags = []models.Tag{} // Ensure it's an empty slice, not null
	} else {
		responsePayload.Tags = tags
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

// DeleteTrafficLogsForTargetHandler handles the DELETE request to remove all logs for a specific target.
func DeleteTrafficLogsForTargetHandler(w http.ResponseWriter, r *http.Request, targetID int64) {
	// targetID is already parsed by the calling router function

	if targetID == 0 {
		logger.Error("API DeleteTrafficLogs: Target ID 0 provided for deleting logs, which is invalid.")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Target ID cannot be zero"})
		return
	}

	// Ensure the target itself exists (optional, but good practice)
	var exists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", targetID).Scan(&exists)
	if err != nil {
		logger.Error("API DeleteTrafficLogs: Error checking existence of target ID %d before deleting logs: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to verify target existence"})
		return
	}
	if !exists {
		logger.Info("API DeleteTrafficLogs: Attempt to delete logs for non-existent target ID %d", targetID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Target not found, no logs to delete"})
		return
	}

	stmt, err := database.DB.Prepare("DELETE FROM http_traffic_log WHERE target_id = ?")
	if err != nil {
		logger.Error("API DeleteTrafficLogs: Error preparing delete statement for http_traffic_log, target_id %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to prepare log deletion query"})
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(targetID)
	if err != nil {
		logger.Error("API DeleteTrafficLogs: Error executing delete on http_traffic_log for target_id %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Failed to delete logs for target"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Info("API DeleteTrafficLogs: Deleted %d proxy logs from http_traffic_log for target_id %d", rowsAffected, targetID)
	w.WriteHeader(http.StatusNoContent)
}

// AnalyzeCommentsHandler handles requests to find comments in a log entry's response body.
func AnalyzeCommentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		HTTPLogID int64 `json:"http_log_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("AnalyzeCommentsHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.HTTPLogID == 0 {
		http.Error(w, "http_log_id is required", http.StatusBadRequest)
		return
	}

	// Fetch the response body for the given log ID
	var responseBody []byte
	err := database.DB.QueryRow("SELECT response_body FROM http_traffic_log WHERE id = ?", req.HTTPLogID).Scan(&responseBody)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Log entry not found", http.StatusNotFound)
		} else {
			logger.Error("AnalyzeCommentsHandler: Error fetching response body for log ID %d: %v", req.HTTPLogID, err)
			http.Error(w, "Failed to fetch log entry data", http.StatusInternalServerError)
		}
		return
	}

	findings, err := database.FindCommentsInText(string(responseBody)) // FindCommentsInText expects a string
	// FindCommentsInText currently doesn't return an error, but good to keep for future.

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(findings) // Send back the array of CommentFinding
}
