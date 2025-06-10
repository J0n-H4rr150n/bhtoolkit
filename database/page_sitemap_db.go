package database

import (
	"database/sql"
	"fmt"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

// CreatePage inserts a new page record into the database.
func CreatePage(page models.Page) (int64, error) {
	// Determine the next display_order for this target_id
	var maxOrder sql.NullInt64
	err := DB.QueryRow("SELECT MAX(display_order) FROM pages WHERE target_id = ?", page.TargetID).Scan(&maxOrder)
	if err != nil && err != sql.ErrNoRows {
		logger.Error("CreatePage: Error getting max display_order for target_id %d: %v", page.TargetID, err)
		return 0, fmt.Errorf("getting max display_order: %w", err)
	}

	nextOrder := 0
	if maxOrder.Valid {
		nextOrder = int(maxOrder.Int64) + 1
	}
	page.DisplayOrder = nextOrder

	stmt, err := DB.Prepare(`INSERT INTO pages
        (target_id, name, description, start_timestamp, created_at, updated_at, display_order)
        VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		logger.Error("CreatePage: Error preparing statement: %v", err)
		return 0, fmt.Errorf("preparing page insert statement: %w", err)
	}
	defer stmt.Close()

	currentTime := time.Now()
	result, err := stmt.Exec(page.TargetID, page.Name, page.Description, page.StartTimestamp, currentTime, currentTime, page.DisplayOrder)
	if err != nil {
		logger.Error("CreatePage: Error executing insert: %v", err)
		return 0, fmt.Errorf("executing page insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Error("CreatePage: Error getting last insert ID: %v", err)
		return 0, fmt.Errorf("getting last insert ID for page: %w", err)
	}
	return id, nil
}

// UpdatePageEndTime sets the end_timestamp for a given page.
func UpdatePageEndTime(pageID int64, endTime time.Time) error {
	stmt, err := DB.Prepare(`UPDATE pages SET end_timestamp = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		logger.Error("UpdatePageEndTime: Error preparing statement: %v", err)
		return fmt.Errorf("preparing page update statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(endTime, time.Now(), pageID)
	if err != nil {
		logger.Error("UpdatePageEndTime: Error executing update for page ID %d: %v", pageID, err)
		return fmt.Errorf("executing page update for page ID %d: %w", pageID, err)
	}
	return nil
}

// AssociateLogsToPage links http_traffic_log entries to a page based on target_id and timestamp range.
// It will only associate the first occurrence (min log id) of each unique (method, url) pair within the timeframe.
func AssociateLogsToPage(pageID int64, targetID int64, startTimestamp time.Time, endTimestamp time.Time) (int64, error) {
	stmt, err := DB.Prepare(`
		INSERT INTO page_http_logs (page_id, http_traffic_log_id)
		SELECT ?, l.id
		FROM http_traffic_log l
		INNER JOIN (
			SELECT MIN(id) as min_id
			FROM http_traffic_log
			WHERE target_id = ? AND timestamp >= ? AND timestamp <= ?
			GROUP BY request_method, request_url 
		) AS unique_logs ON l.id = unique_logs.min_id
		WHERE l.target_id = ? AND l.timestamp >= ? AND l.timestamp <= ?
		ON CONFLICT(page_id, http_traffic_log_id) DO NOTHING
	`)
	if err != nil {
		logger.Error("AssociateLogsToPage: Error preparing statement: %v", err)
		return 0, fmt.Errorf("preparing associate logs statement: %w", err)
	}
	defer stmt.Close()
	// Parameters are: pageID, targetID (subquery), start (subquery), end (subquery), targetID (outer), start (outer), end (outer)
	result, err := stmt.Exec(pageID, targetID, startTimestamp, endTimestamp, targetID, startTimestamp, endTimestamp)
	if err != nil {
		logger.Error("AssociateLogsToPage: Error executing association for page ID %d: %v", pageID, err)
		return 0, fmt.Errorf("executing log association for page ID %d: %w", pageID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("AssociateLogsToPage: Error getting rows affected for page ID %d: %v", pageID, err)
	}
	return rowsAffected, nil
}

// GetPagesForTarget retrieves all pages recorded for a specific target_id.
func GetPagesForTarget(targetID int64) ([]models.Page, error) {
	rows, err := DB.Query(`SELECT id, target_id, name, description, start_timestamp, end_timestamp, created_at, updated_at, display_order
                           FROM pages WHERE target_id = ? ORDER BY display_order ASC, start_timestamp DESC`, targetID)
	if err != nil {
		logger.Error("GetPagesForTarget: Error querying pages for target %d: %v", targetID, err)
		return nil, fmt.Errorf("querying pages for target %d: %w", targetID, err)
	}
	defer rows.Close()
	var pages []models.Page
	for rows.Next() {
		var p models.Page
		if err := rows.Scan(&p.ID, &p.TargetID, &p.Name, &p.Description, &p.StartTimestamp, &p.EndTimestamp, &p.CreatedAt, &p.UpdatedAt, &p.DisplayOrder); err != nil {
			logger.Error("GetPagesForTarget: Error scanning page for target %d: %v", targetID, err)
			return nil, fmt.Errorf("scanning page for target %d: %w", targetID, err)
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// GetLogsForPage retrieves all HTTPTrafficLog entries associated with a specific page_id.
func GetLogsForPage(pageID int64) ([]models.HTTPTrafficLog, error) {
	query := `
		SELECT 
			htl.id, htl.target_id, htl.timestamp, htl.request_method, htl.request_url, 
			htl.request_http_version, htl.request_headers, htl.request_body, 
			htl.response_status_code, htl.response_reason_phrase, htl.response_http_version, 
			htl.response_headers, htl.response_body, htl.response_content_type, 
			htl.response_body_size, htl.duration_ms, htl.client_ip, htl.server_ip, 
			htl.is_https, htl.is_page_candidate, htl.notes, htl.is_favorite
		FROM http_traffic_log htl
		JOIN page_http_logs phl ON htl.id = phl.http_traffic_log_id
		WHERE phl.page_id = ?
		ORDER BY htl.timestamp ASC
	`
	rows, err := DB.Query(query, pageID)
	if err != nil {
		logger.Error("GetLogsForPage: Error querying logs for page %d: %v", pageID, err)
		return nil, fmt.Errorf("querying logs for page %d: %w", pageID, err)
	}
	defer rows.Close()

	var logs []models.HTTPTrafficLog
	for rows.Next() {
		var h models.HTTPTrafficLog
		// Use sql.Null types for scanning nullable fields from DB
		var targetID sql.NullInt64
		var requestBody, responseBody sql.NullString // Assuming BLOBs are handled as strings by driver or use sql.NullBytes
		var responseStatusCode sql.NullInt64
		var responseReasonPhrase, responseHTTPVersion, responseHeaders, responseContentType sql.NullString
		var responseBodySize, durationMs sql.NullInt64
		var clientIP, serverIP, notes sql.NullString

		err := rows.Scan(
			&h.ID, &targetID, &h.Timestamp, &h.RequestMethod, &h.RequestURL,
			&h.RequestHTTPVersion, &h.RequestHeaders, &requestBody,
			&responseStatusCode, &responseReasonPhrase, &responseHTTPVersion,
			&responseHeaders, &responseBody, &responseContentType,
			&responseBodySize, &durationMs, &clientIP, &serverIP,
			&h.IsHTTPS, &h.IsPageCandidate, &notes, &h.IsFavorite,
		)
		if err != nil {
			logger.Error("GetLogsForPage: Error scanning log for page %d: %v", pageID, err)
			return nil, fmt.Errorf("scanning log for page %d: %w", pageID, err)
		}

		// Assign scanned nullable values to the struct fields
		if targetID.Valid {
			h.TargetID = &targetID.Int64
		}
		h.RequestBody = []byte(requestBody.String)           // Convert back to []byte if needed, or adjust model
		h.ResponseStatusCode = int(responseStatusCode.Int64) // Assuming 0 if null
		h.ResponseReasonPhrase = responseReasonPhrase.String
		h.ResponseHTTPVersion = responseHTTPVersion.String
		h.ResponseHeaders = responseHeaders.String
		h.ResponseBody = []byte(responseBody.String) // Convert back to []byte
		h.ResponseContentType = responseContentType.String
		h.ResponseBodySize = responseBodySize.Int64 // Assuming 0 if null
		h.DurationMs = durationMs.Int64             // Assuming 0 if null
		h.ClientIP = clientIP.String
		h.ServerIP = serverIP.String
		h.Notes = notes.String

		logs = append(logs, h)
	}
	return logs, rows.Err()
}

// UpdatePagesOrder updates the display_order for a list of pages.
// It expects a map where keys are page IDs and values are their new display order.
func UpdatePagesOrder(pageOrders map[int64]int) error {
	tx, err := DB.Begin()
	if err != nil {
		logger.Error("UpdatePagesOrder: Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	stmt, err := tx.Prepare("UPDATE pages SET display_order = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		tx.Rollback() // Rollback on error
		logger.Error("UpdatePagesOrder: Failed to prepare statement: %v", err)
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	updatedAt := time.Now()
	for pageID, order := range pageOrders {
		if _, err := stmt.Exec(order, updatedAt, pageID); err != nil {
			tx.Rollback() // Rollback on error
			logger.Error("UpdatePagesOrder: Failed to update order for page ID %d: %v", pageID, err)
			return fmt.Errorf("failed to update order for page ID %d: %w", pageID, err)
		}
	}

	logger.Info("Successfully updated display order for %d pages.", len(pageOrders))
	return tx.Commit()
}

// DeletePage removes a page and its associated log entries (via CASCADE) from the database.
func DeletePage(pageID int64) error {
	stmt, err := DB.Prepare("DELETE FROM pages WHERE id = ?")
	if err != nil {
		logger.Error("DeletePage: Error preparing statement for page ID %d: %v", pageID, err)
		return fmt.Errorf("preparing delete statement for page ID %d: %w", pageID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(pageID)
	if err != nil {
		logger.Error("DeletePage: Error executing delete for page ID %d: %v", pageID, err)
		return fmt.Errorf("executing delete for page ID %d: %w", pageID, err)
	}

	rowsAffected, _ := result.RowsAffected() // Error check for RowsAffected is optional here
	logger.Info("DeletePage: Successfully deleted page ID %d, rows affected: %d", pageID, rowsAffected)
	return nil
}

// UpdatePageDetails updates the name and/or description of a specific page.
func UpdatePageDetails(pageID int64, name string, description sql.NullString) error {
	stmt, err := DB.Prepare("UPDATE pages SET name = ?, description = ?, updated_at = ? WHERE id = ?")
	if err != nil {
		logger.Error("UpdatePageDetails: Error preparing statement for page ID %d: %v", pageID, err)
		return fmt.Errorf("preparing update statement for page ID %d: %w", pageID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(name, description, time.Now(), pageID)
	if err != nil {
		logger.Error("UpdatePageDetails: Error executing update for page ID %d: %v", pageID, err)
		return fmt.Errorf("executing update for page ID %d: %w", pageID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("UpdatePageDetails: Error getting rows affected for page ID %d: %v", pageID, err)
		// Not returning error here as the update might have succeeded.
	}
	if rowsAffected == 0 {
		logger.Info("UpdatePageDetails: No rows affected when updating page ID %d. Page might not exist.", pageID)
		return fmt.Errorf("no page found with ID %d to update", pageID) // Or return nil if "not found" is not an error for update
	}
	logger.Info("UpdatePageDetails: Successfully updated details for page ID %d", pageID)
	return nil
}
