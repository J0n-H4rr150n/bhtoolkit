package database

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
	"toolkit/logger"
	"toolkit/models"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// CreateOrUpdateParameterizedURL inserts a new parameterized URL if it doesn't exist (based on unique constraint)
// or updates the last_seen_at timestamp and http_traffic_log_id (if new one is more recent) if it does.
// It returns the ID of the entry and a boolean indicating if a new entry was created.
func CreateOrUpdateParameterizedURL(pUrl models.ParameterizedURL) (id int64, created bool, err error) {
	// Ensure param_keys are sorted for consistent storage and querying
	keys := strings.Split(pUrl.ParamKeys, ",")
	sort.Strings(keys)
	pUrl.ParamKeys = strings.Join(keys, ",")

	tx, err := DB.Begin()
	if err != nil {
		logger.Error("CreateOrUpdateParameterizedURL: Failed to begin transaction: %v", err)
		return 0, false, err
	}
	defer tx.Rollback() // Rollback if not committed

	var existingID int64
	var existingLogID int64
	var existingLastSeen time.Time

	query := `SELECT id, http_traffic_log_id, last_seen_at FROM parameterized_urls
	          WHERE target_id = ? AND request_method = ? AND request_path = ? AND param_keys = ?`
	err = tx.QueryRow(query, pUrl.TargetID, pUrl.RequestMethod, pUrl.RequestPath, pUrl.ParamKeys).Scan(&existingID, &existingLogID, &existingLastSeen)

	if err != nil && err != sql.ErrNoRows {
		logger.Error("CreateOrUpdateParameterizedURL: Error checking for existing entry: %v", err)
		return 0, false, err
	}

	currentTime := time.Now()

	if err == sql.ErrNoRows { // Entry does not exist, create it
		insertQuery := `INSERT INTO parameterized_urls (target_id, http_traffic_log_id, request_method, request_path, param_keys, example_full_url, notes, discovered_at, last_seen_at)
		                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		result, err := tx.Exec(insertQuery, pUrl.TargetID, pUrl.HTTPTrafficLogID, pUrl.RequestMethod, pUrl.RequestPath, pUrl.ParamKeys, pUrl.ExampleFullURL, pUrl.Notes, currentTime, currentTime)
		if err != nil {
			logger.Error("CreateOrUpdateParameterizedURL: Error inserting new entry: %v", err)
			return 0, false, err
		}
		id, _ = result.LastInsertId()
		created = true
	} else { // Entry exists, update last_seen_at and potentially http_traffic_log_id and example_full_url
		id = existingID
		updateQuery := `UPDATE parameterized_urls SET last_seen_at = ?`
		args := []interface{}{currentTime}

		// Optionally update example_full_url and http_traffic_log_id if the current log is more recent
		// This assumes http_traffic_log_id is somewhat sequential with time.
		// A more robust way would be to compare timestamps of the log entries if available.
		if pUrl.HTTPTrafficLogID > existingLogID {
			updateQuery += `, http_traffic_log_id = ?, example_full_url = ?`
			args = append(args, pUrl.HTTPTrafficLogID, pUrl.ExampleFullURL)
		}
		updateQuery += ` WHERE id = ?`
		args = append(args, id)

		_, err := tx.Exec(updateQuery, args...)
		if err != nil {
			logger.Error("CreateOrUpdateParameterizedURL: Error updating existing entry ID %d: %v", id, err)
			return 0, false, err
		}
		created = false
	}

	if err = tx.Commit(); err != nil {
		logger.Error("CreateOrUpdateParameterizedURL: Failed to commit transaction: %v", err)
		return 0, false, err
	}
	return id, created, nil
}

// GetParameterizedURLs retrieves a paginated, sorted, and filtered list of parameterized URLs.
func GetParameterizedURLs(params models.ParameterizedURLFilters) ([]models.ParameterizedURL, int, error) {
	var urls []models.ParameterizedURL
	var totalRecords int

	baseQuery := "FROM parameterized_urls WHERE target_id = ?"
	args := []interface{}{params.TargetID}

	if params.RequestMethod != "" {
		baseQuery += " AND request_method = ?"
		args = append(args, params.RequestMethod)
	}
	if params.PathSearch != "" {
		baseQuery += " AND request_path LIKE ?"
		args = append(args, "%"+params.PathSearch+"%")
	}
	if params.ParamKeysSearch != "" {
		baseQuery += " AND param_keys LIKE ?"
		args = append(args, "%"+params.ParamKeysSearch+"%")
	}

	countQuery := "SELECT COUNT(*) " + baseQuery
	err := DB.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		logger.Error("GetParameterizedURLs: Error counting records: %v", err)
		return nil, 0, err
	}

	sortOrder := "DESC"
	if strings.ToUpper(params.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	orderBy := "discovered_at" // Default sort
	if params.SortBy != "" {
		// Validate SortBy to prevent SQL injection if it's dynamic
		validSortColumns := map[string]string{"request_path": "request_path", "request_method": "request_method", "discovered_at": "discovered_at", "last_seen_at": "last_seen_at"}
		if col, ok := validSortColumns[params.SortBy]; ok {
			orderBy = col
		}
	}

	query := fmt.Sprintf("SELECT id, target_id, http_traffic_log_id, request_method, request_path, param_keys, example_full_url, notes, discovered_at, last_seen_at %s ORDER BY %s %s LIMIT ? OFFSET ?", baseQuery, orderBy, sortOrder)
	args = append(args, params.Limit, (params.Page-1)*params.Limit)

	rows, err := DB.Query(query, args...)
	if err != nil {
		logger.Error("GetParameterizedURLs: Error querying records: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var u models.ParameterizedURL
		if err := rows.Scan(&u.ID, &u.TargetID, &u.HTTPTrafficLogID, &u.RequestMethod, &u.RequestPath, &u.ParamKeys, &u.ExampleFullURL, &u.Notes, &u.DiscoveredAt, &u.LastSeenAt); err != nil {
			logger.Error("GetParameterizedURLs: Error scanning row: %v", err)
			return nil, 0, err
		}
		urls = append(urls, u)
	}
	return urls, totalRecords, nil
}
