package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

// DistinctDomainValues holds slices of distinct values for filter dropdowns.
type DistinctDomainValues struct {
	DistinctHttpStatusCodes []sql.NullInt64
	DistinctHttpServers     []sql.NullString
	DistinctHttpTechs       []sql.NullString
}

// GetDomains retrieves a paginated list of domains for a specific target, with filtering and sorting based on DomainFilters.
// It now also returns distinct values for certain filterable columns.
func GetDomains(filters models.DomainFilters) ([]models.Domain, int64, *DistinctDomainValues, error) {
	if DB == nil {
		return nil, 0, nil, errors.New("database connection is not initialized")
	}

	var domains []models.Domain
	var totalRecords int64
	args := []interface{}{filters.TargetID}
	countArgs := []interface{}{filters.TargetID} // Initialize countArgs with TargetID

	whereClause := "WHERE target_id = ?"

	// Apply search filters
	if filters.DomainNameSearch != "" {
		whereClause += " AND LOWER(domain_name) LIKE LOWER(?)"
		args = append(args, "%"+filters.DomainNameSearch+"%")
		countArgs = append(countArgs, "%"+filters.DomainNameSearch+"%") // Ensure this is added
	}
	if filters.SourceSearch != "" {
		whereClause += " AND LOWER(source) LIKE LOWER(?)"
		args = append(args, "%"+filters.SourceSearch+"%")
		countArgs = append(countArgs, "%"+filters.SourceSearch+"%")
	}

	if filters.IsInScope != nil {
		whereClause += " AND is_in_scope = ?"
		args = append(args, *filters.IsInScope)
		countArgs = append(countArgs, *filters.IsInScope)
	}
	// Only apply is_favorite filter if the pointer is non-nil AND the value is true
	if filters.IsFavorite != nil {
		if *filters.IsFavorite { // Check if the boolean value is true
			whereClause += " AND is_favorite = ?"
			args = append(args, true)           // Add the boolean value 'true' to args
			countArgs = append(countArgs, true) // Add to countArgs as well
		}
	}

	// Temporary whereClause and args for distinct value queries,
	// so they are not affected by the specific column filters applied later for the main query.
	distinctWhereClause := whereClause
	distinctCountArgs := make([]interface{}, len(countArgs))
	copy(distinctCountArgs, countArgs)

	if filters.HttpxScanStatus == "scanned" {
		// This filter applies to both distinct value queries and the main record query
		additionalFilter := " AND (httpx_full_json IS NOT NULL AND httpx_full_json != '')"
		whereClause += additionalFilter
		distinctWhereClause += additionalFilter
		// No new arguments needed for countArgs or distinctCountArgs for this filter
	} else if filters.HttpxScanStatus == "not_scanned" {
		additionalFilter := " AND (httpx_full_json IS NULL OR httpx_full_json = '')"
		whereClause += additionalFilter
		distinctWhereClause += additionalFilter
		// No new arguments needed for countArgs or distinctCountArgs for this filter
	}

	// --- Fetch Distinct Values ---
	distinctValues := &DistinctDomainValues{}
	var err error

	// Distinct HTTP Status Codes
	distinctQueryStatusCode := "SELECT DISTINCT http_status_code FROM domains " + distinctWhereClause + " AND http_status_code IS NOT NULL ORDER BY http_status_code ASC"
	rowsDistinctStatus, err := DB.Query(distinctQueryStatusCode, distinctCountArgs...)
	if err != nil {
		logger.Error("Error fetching distinct http_status_code: %v. Query: %s, Args: %v", err, distinctQueryStatusCode, distinctCountArgs)
	} else {
		for rowsDistinctStatus.Next() {
			var val sql.NullInt64
			if scanErr := rowsDistinctStatus.Scan(&val); scanErr == nil {
				distinctValues.DistinctHttpStatusCodes = append(distinctValues.DistinctHttpStatusCodes, val)
			}
		}
		rowsDistinctStatus.Close()
	}

	// Distinct HTTP Servers
	distinctQueryServer := "SELECT DISTINCT http_server FROM domains " + distinctWhereClause + " AND http_server IS NOT NULL AND http_server != '' ORDER BY http_server ASC"
	rowsDistinctServer, err := DB.Query(distinctQueryServer, distinctCountArgs...)
	if err != nil {
		logger.Error("Error fetching distinct http_server: %v. Query: %s, Args: %v", err, distinctQueryServer, distinctCountArgs)
	} else {
		for rowsDistinctServer.Next() {
			var val sql.NullString
			if scanErr := rowsDistinctServer.Scan(&val); scanErr == nil {
				distinctValues.DistinctHttpServers = append(distinctValues.DistinctHttpServers, val)
			}
		}
		rowsDistinctServer.Close()
	}

	// Distinct HTTP Tech
	distinctQueryTech := "SELECT DISTINCT http_tech FROM domains " + distinctWhereClause + " AND http_tech IS NOT NULL AND http_tech != '' ORDER BY http_tech ASC"
	rowsDistinctTech, err := DB.Query(distinctQueryTech, distinctCountArgs...)
	if err != nil {
		logger.Error("Error fetching distinct http_tech: %v. Query: %s, Args: %v", err, distinctQueryTech, distinctCountArgs)
	} else {
		for rowsDistinctTech.Next() {
			var val sql.NullString
			if scanErr := rowsDistinctTech.Scan(&val); scanErr == nil {
				distinctValues.DistinctHttpTechs = append(distinctValues.DistinctHttpTechs, val)
			}
		}
		rowsDistinctTech.Close()
	}
	// --- End Fetch Distinct Values ---

	// Now, apply the specific column filters to the main whereClause for fetching records
	// and also update countArgs for the final count query.
	finalCountArgs := make([]interface{}, len(countArgs)) // Create a new slice for final count args
	copy(finalCountArgs, countArgs)

	if filters.FilterHTTPStatusCode != "" {
		if strings.ToUpper(filters.FilterHTTPStatusCode) == "NULL" || strings.ToUpper(filters.FilterHTTPStatusCode) == "N/A" {
			whereClause += " AND http_status_code IS NULL"
		} else {
			statusCodeInt, err := strconv.ParseInt(filters.FilterHTTPStatusCode, 10, 64)
			if err == nil {
				whereClause += " AND http_status_code = ?"
				args = append(args, statusCodeInt)
				finalCountArgs = append(finalCountArgs, statusCodeInt) // Add to final count args
			} else {
				logger.Warn("GetDomains: Invalid non-numeric http_status_code filter value '%s', ignoring filter.", filters.FilterHTTPStatusCode)
			}
		}
	}
	if filters.FilterHTTPServer != "" {
		if strings.ToUpper(filters.FilterHTTPServer) == "NULL" || strings.ToUpper(filters.FilterHTTPServer) == "N/A" {
			whereClause += " AND (http_server IS NULL OR http_server = '')"
		} else {
			whereClause += " AND http_server = ?"
			args = append(args, filters.FilterHTTPServer)
			finalCountArgs = append(finalCountArgs, filters.FilterHTTPServer) // Add to final count args
		}
	}
	if filters.FilterHTTPTech != "" {
		if strings.ToUpper(filters.FilterHTTPTech) == "NULL" || strings.ToUpper(filters.FilterHTTPTech) == "N/A" {
			whereClause += " AND (http_tech IS NULL OR http_tech = '')"
		} else {
			whereClause += " AND http_tech = ?"
			args = append(args, filters.FilterHTTPTech)
			finalCountArgs = append(finalCountArgs, filters.FilterHTTPTech) // Add to final count args
		}
	}

	countQuery := "SELECT COUNT(*) FROM domains " + whereClause
	err = DB.QueryRow(countQuery, finalCountArgs...).Scan(&totalRecords) // Use finalCountArgs
	if err != nil {
		logger.Error("Error counting domains: %v. Query: %s, Args: %v", err, countQuery, finalCountArgs)
		return nil, 0, distinctValues, fmt.Errorf("counting domains failed: %w", err)
	}

	selectQuery := "SELECT id, target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite, http_status_code, http_content_length, http_title, http_server, http_tech, httpx_full_json FROM domains " + whereClause

	allowedSortCols := map[string]bool{
		"id": true, "domain_name": true, "source": true, "is_in_scope": true,
		"is_wildcard_scope": true, "notes": true, "created_at": true, "updated_at": true,
		"is_favorite": true, "http_status_code": true, "http_content_length": true,
		"http_title": true, "http_server": true, "http_tech": true,
	}
	if !allowedSortCols[filters.SortBy] {
		filters.SortBy = "domain_name"
	}
	if strings.ToUpper(filters.SortOrder) != "ASC" && strings.ToUpper(filters.SortOrder) != "DESC" {
		filters.SortOrder = "ASC"
	}
	orderByClause := fmt.Sprintf("ORDER BY %s %s, id %s", filters.SortBy, filters.SortOrder, filters.SortOrder)

	selectQuery += " " + orderByClause // Add space before ORDER BY

	if filters.Limit > 0 {
		offset := (filters.Page - 1) * filters.Limit
		selectQuery += " LIMIT ? OFFSET ?" // Add space before LIMIT
		args = append(args, filters.Limit, offset)
	}

	rows, err := DB.Query(selectQuery, args...)
	if err != nil {
		logger.Error("Error querying domains: %v. Query: %s, Args: %v", err, selectQuery, args)
		return nil, 0, distinctValues, fmt.Errorf("querying domains failed (query: %s): %w", selectQuery, err)
	}
	defer rows.Close()

	for rows.Next() {
		var d models.Domain
		var createdAtStr string
		var updatedAtStr string
		if err := rows.Scan(&d.ID, &d.TargetID, &d.DomainName, &d.Source, &d.IsInScope, &d.IsWildcardScope, &d.Notes, &createdAtStr, &updatedAtStr, &d.IsFavorite, &d.HTTPStatusCode, &d.HTTPContentLength, &d.HTTPTitle, &d.HTTPServer, &d.HTTPTech, &d.HttpxFullJson); err != nil {
			logger.Error("Error scanning domain row: %v", err)
			return nil, 0, distinctValues, fmt.Errorf("scanning domain row failed: %w", err)
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		d.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		domains = append(domains, d)
	}

	if err = rows.Err(); err != nil {
		logger.Error("Error iterating domain rows: %v", err)
		return nil, 0, distinctValues, fmt.Errorf("iterating domain rows failed: %w", err)
	}
	return domains, totalRecords, distinctValues, nil
}

// CreateDomain creates a new domain entry in the database.
func CreateDomain(domain models.Domain) (int64, error) {
	if DB == nil {
		return 0, errors.New("database connection is not initialized")
	}
	if strings.TrimSpace(domain.DomainName) == "" {
		return 0, errors.New("domain_name cannot be empty")
	}
	var existingID int64
	err := DB.QueryRow("SELECT id FROM domains WHERE target_id = ? AND domain_name = ?", domain.TargetID, domain.DomainName).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		logger.Error("Error checking for existing domain %s for target %d: %v", domain.DomainName, domain.TargetID, err)
		return 0, fmt.Errorf("checking for existing domain failed: %w", err)
	}
	if existingID > 0 {
		return 0, fmt.Errorf("domain '%s' already exists for this target", domain.DomainName)
	}
	stmt, err := DB.Prepare("INSERT INTO domains (target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)")
	if err != nil {
		logger.Error("Error preparing statement to create domain: %v", err)
		return 0, fmt.Errorf("preparing domain creation failed: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(domain.TargetID, domain.DomainName, domain.Source, domain.IsInScope, domain.IsWildcardScope, domain.Notes, domain.IsFavorite)
	if err != nil {
		logger.Error("Error executing insert for domain %s: %v", domain.DomainName, err)
		return 0, fmt.Errorf("domain insertion failed: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		logger.Error("Error getting last insert ID for domain %s: %v", domain.DomainName, err)
		return 0, fmt.Errorf("getting last insert ID failed: %w", err)
	}
	logger.Info("Domain created: ID %d, Name '%s', TargetID %d", id, domain.DomainName, domain.TargetID)
	return id, nil
}

// UpdateDomain updates an existing domain's details.
func UpdateDomain(domain models.Domain) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	stmt, err := DB.Prepare("UPDATE domains SET source = ?, is_in_scope = ?, notes = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	if err != nil {
		logger.Error("Error preparing statement to update domain ID %d: %v", domain.ID, err)
		return fmt.Errorf("preparing domain update failed: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(domain.Source, domain.IsInScope, domain.Notes, domain.ID)
	if err != nil {
		logger.Error("Error executing update for domain ID %d: %v", domain.ID, err)
		return fmt.Errorf("domain update execution failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected for domain update (ID %d): %v", domain.ID, err)
		return fmt.Errorf("checking rows affected for domain update failed: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("domain with ID %d not found for update", domain.ID)
	}
	logger.Info("Domain updated: ID %d", domain.ID)
	return nil
}

// UpdateDomainWithHttpxResult updates a domain entry with results from an httpx scan.
func UpdateDomainWithHttpxResult(domain models.Domain) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	stmt, err := DB.Prepare(`
		UPDATE domains
		SET http_status_code = ?, http_content_length = ?, http_title = ?,
		    http_server = ?, http_tech = ?, httpx_full_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)
	if err != nil {
		logger.Error("Error preparing statement to update domain with httpx results (ID %d): %v", domain.ID, err)
		return fmt.Errorf("preparing httpx results update failed: %w", err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(
		domain.HTTPStatusCode, domain.HTTPContentLength, domain.HTTPTitle,
		domain.HTTPServer, domain.HTTPTech, domain.HttpxFullJson, domain.ID,
	)
	if err != nil {
		logger.Error("Error executing update for domain with httpx results (ID %d): %v", domain.ID, err)
		return fmt.Errorf("httpx results update execution failed: %w", err)
	}
	return nil
}

// DeleteDomain deletes a domain by its ID.
func DeleteDomain(id int64) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	stmt, err := DB.Prepare("DELETE FROM domains WHERE id = ?")
	if err != nil {
		logger.Error("Error preparing statement to delete domain ID %d: %v", id, err)
		return fmt.Errorf("preparing domain deletion failed: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(id)
	if err != nil {
		logger.Error("Error executing delete for domain ID %d: %v", id, err)
		return fmt.Errorf("domain deletion execution failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected for domain deletion (ID %d): %v", id, err)
		return fmt.Errorf("checking rows affected for domain deletion failed: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("domain with ID %d not found for deletion", id)
	}
	logger.Info("Domain deleted: ID %d", id)
	return nil
}

// DeleteAllDomainsForTarget deletes all domains associated with a specific target_id.
func DeleteAllDomainsForTarget(targetID int64) (int64, error) {
	if DB == nil {
		return 0, errors.New("database connection is not initialized")
	}
	stmt, err := DB.Prepare("DELETE FROM domains WHERE target_id = ?")
	if err != nil {
		logger.Error("Error preparing statement to delete all domains for target_id %d: %v", targetID, err)
		return 0, fmt.Errorf("preparing delete all domains failed: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(targetID)
	if err != nil {
		logger.Error("Error executing delete all domains for target_id %d: %v", targetID, err)
		return 0, fmt.Errorf("executing delete all domains failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected for delete all domains (target_id %d): %v", targetID, err)
		return 0, fmt.Errorf("checking rows affected for delete all domains failed: %w", err)
	}
	logger.Info("Deleted %d domains for target_id %d", rowsAffected, targetID)
	return rowsAffected, nil
}

// SetDomainFavoriteStatus updates the favorite status of a domain.
func SetDomainFavoriteStatus(domainID int64, isFavorite bool) error {
	if DB == nil {
		return errors.New("database connection is not initialized")
	}
	stmt, err := DB.Prepare("UPDATE domains SET is_favorite = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?")
	if err != nil {
		logger.Error("Error preparing statement to update domain favorite status for ID %d: %v", domainID, err)
		return fmt.Errorf("preparing favorite status update failed: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(isFavorite, domainID)
	if err != nil {
		logger.Error("Error executing update for domain favorite status (ID %d): %v", domainID, err)
		return fmt.Errorf("executing favorite status update failed: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("Error getting rows affected for domain favorite status update (ID %d): %v", domainID, err)
		return fmt.Errorf("checking rows affected for favorite status update failed: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("domain with ID %d not found for favorite status update", domainID)
	}
	logger.Info("Updated favorite status for domain ID %d to %t", domainID, isFavorite)
	return nil
}

// GetDomainIDsByFilters retrieves the IDs of all domains matching the provided filters for a specific target.
func GetDomainIDsByFilters(filters models.DomainFilters) ([]int64, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}

	args := []interface{}{filters.TargetID}
	whereClause := "WHERE target_id = ?"

	if filters.DomainNameSearch != "" {
		whereClause += " AND LOWER(domain_name) LIKE LOWER(?)"
		args = append(args, "%"+filters.DomainNameSearch+"%")
	}
	if filters.SourceSearch != "" {
		whereClause += " AND LOWER(source) LIKE LOWER(?)"
		args = append(args, "%"+filters.SourceSearch+"%")
	}
	if filters.IsInScope != nil {
		whereClause += " AND is_in_scope = ?"
		args = append(args, *filters.IsInScope)
	}
	// Only apply is_favorite filter if the pointer is non-nil AND the value is true
	if filters.IsFavorite != nil {
		if *filters.IsFavorite { // Check if the boolean value is true
			whereClause += " AND is_favorite = ?"
			args = append(args, true) // Add the boolean value 'true' to args
		}
	}
	if filters.HttpxScanStatus == "scanned" {
		whereClause += " AND (httpx_full_json IS NOT NULL AND httpx_full_json != '')"
	} else if filters.HttpxScanStatus == "not_scanned" {
		whereClause += " AND (httpx_full_json IS NULL OR httpx_full_json = '')"
	}
	if filters.FilterHTTPStatusCode != "" {
		if strings.ToUpper(filters.FilterHTTPStatusCode) == "NULL" || strings.ToUpper(filters.FilterHTTPStatusCode) == "N/A" {
			whereClause += " AND http_status_code IS NULL"
		} else {
			statusCodeInt, err := strconv.ParseInt(filters.FilterHTTPStatusCode, 10, 64)
			if err == nil {
				whereClause += " AND http_status_code = ?"
				args = append(args, statusCodeInt)
			} else {
				logger.Warn("GetDomainIDsByFilters: Invalid non-numeric http_status_code filter value '%s', ignoring filter.", filters.FilterHTTPStatusCode)
			}
		}
	}
	if filters.FilterHTTPServer != "" {
		if strings.ToUpper(filters.FilterHTTPServer) == "NULL" || strings.ToUpper(filters.FilterHTTPServer) == "N/A" {
			whereClause += " AND (http_server IS NULL OR http_server = '')"
		} else {
			whereClause += " AND http_server = ?"
			args = append(args, filters.FilterHTTPServer)
		}
	}
	if filters.FilterHTTPTech != "" {
		if strings.ToUpper(filters.FilterHTTPTech) == "NULL" || strings.ToUpper(filters.FilterHTTPTech) == "N/A" {
			whereClause += " AND (http_tech IS NULL OR http_tech = '')"
		} else {
			whereClause += " AND http_tech = ?"
			args = append(args, filters.FilterHTTPTech)
		}
	}

	logger.Debug("GetDomainIDsByFilters: Executing query: SELECT id FROM domains %s with args: %v", whereClause, args)
	query := "SELECT id FROM domains " + whereClause

	rows, err := DB.Query(query, args...)
	if err != nil {
		logger.Error("GetDomainIDsByFilters: Error querying domain IDs: %v. Query: %s, Args: %v", err, query, args)
		return nil, fmt.Errorf("querying domain IDs by filters failed: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			logger.Error("GetDomainIDsByFilters: Error scanning domain ID row: %v", err)
			return nil, fmt.Errorf("scanning domain ID row failed: %w", err)
		}
		ids = append(ids, id)
	}
	logger.Debug("GetDomainIDsByFilters: Found %d domain IDs matching filters.", len(ids))
	return ids, rows.Err()
}

// GetDomainsByIDs retrieves multiple domain entries by their IDs.
func GetDomainsByIDs(ids []int64) ([]models.Domain, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}
	if len(ids) == 0 {
		return []models.Domain{}, nil
	}
	logger.Debug("GetDomainsByIDs: Attempting to fetch %d domain IDs.", len(ids))
	query := `SELECT id, target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite,
	                 http_status_code, http_content_length, http_title, http_server, http_tech, httpx_full_json
	          FROM domains WHERE id IN (?` + strings.Repeat(",?", len(ids)-1) + `)`
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := DB.Query(query, args...)
	if err != nil {
		logger.Error("GetDomainsByIDs: Error querying domains by IDs: %v", err)
		return nil, fmt.Errorf("querying domains by IDs failed: %w", err)
	}
	defer rows.Close()
	var domains []models.Domain
	for rows.Next() {
		var d models.Domain
		var createdAtStr, updatedAtStr string
		if err := rows.Scan(&d.ID, &d.TargetID, &d.DomainName, &d.Source, &d.IsInScope, &d.IsWildcardScope, &d.Notes, &createdAtStr, &updatedAtStr, &d.IsFavorite, &d.HTTPStatusCode, &d.HTTPContentLength, &d.HTTPTitle, &d.HTTPServer, &d.HTTPTech, &d.HttpxFullJson); err != nil {
			logger.Error("GetDomainsByIDs: Error scanning domain row: %v", err)
			return nil, fmt.Errorf("scanning domain row failed: %w", err)
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		d.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		domains = append(domains, d)
	}
	logger.Debug("GetDomainsByIDs: Fetched %d domain objects for %d input IDs.", len(domains), len(ids))
	return domains, rows.Err()
}

// GetDomainByID retrieves a single domain by its ID.
func GetDomainByID(id int64) (*models.Domain, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}
	var d models.Domain
	var createdAtStr, updatedAtStr string
	query := `SELECT id, target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite,
	                 http_status_code, http_content_length, http_title, http_server, http_tech, httpx_full_json
	          FROM domains WHERE id = ?`
	err := DB.QueryRow(query, id).Scan(
		&d.ID, &d.TargetID, &d.DomainName, &d.Source, &d.IsInScope, &d.IsWildcardScope, &d.Notes, &createdAtStr, &updatedAtStr, &d.IsFavorite,
		&d.HTTPStatusCode, &d.HTTPContentLength, &d.HTTPTitle, &d.HTTPServer, &d.HTTPTech, &d.HttpxFullJson,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("domain with ID %d not found", id)
		}
		logger.Error("Error scanning domain by ID %d: %v", id, err)
		return nil, fmt.Errorf("querying domain by ID failed: %w", err)
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	d.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
	return &d, nil
}

// FavoriteAllFilteredDomainsDB marks all domains matching the filters for a target as favorite.
func FavoriteAllFilteredDomainsDB(targetID int64, domainNameSearch, sourceSearch string, isInScope *bool) (int64, error) {
	if DB == nil {
		return 0, errors.New("database connection is not initialized")
	}
	query := "UPDATE domains SET is_favorite = TRUE, updated_at = CURRENT_TIMESTAMP WHERE target_id = ?"
	args := []interface{}{targetID}
	if domainNameSearch != "" {
		query += " AND LOWER(domain_name) LIKE LOWER(?)"
		args = append(args, "%"+domainNameSearch+"%")
	}
	if sourceSearch != "" {
		query += " AND LOWER(source) LIKE LOWER(?)"
		args = append(args, "%"+sourceSearch+"%")
	}
	if isInScope != nil {
		query += " AND is_in_scope = ?"
		args = append(args, *isInScope)
	}
	stmt, err := DB.Prepare(query)
	if err != nil {
		logger.Error("Error preparing statement for FavoriteAllFilteredDomainsDB (target %d): %v. Query: %s", targetID, err, query)
		return 0, fmt.Errorf("preparing favorite all filtered domains failed: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(args...)
	if err != nil {
		logger.Error("Error executing update for FavoriteAllFilteredDomainsDB (target %d): %v. Query: %s, Args: %v", targetID, err, query, args)
		return 0, fmt.Errorf("executing favorite all filtered domains failed: %w", err)
	}
	return result.RowsAffected()
}
