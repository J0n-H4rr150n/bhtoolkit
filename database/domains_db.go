package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

// GetDomains retrieves a paginated list of domains for a specific target, with filtering and sorting based on DomainFilters.
func GetDomains(filters models.DomainFilters) ([]models.Domain, int64, error) {
	if DB == nil {
		return nil, 0, errors.New("database connection is not initialized")
	}

	var domains []models.Domain
	var totalRecords int64
	args := []interface{}{filters.TargetID}
	countArgs := []interface{}{filters.TargetID}

	baseQuery := "FROM domains WHERE target_id = ? "
	countQuery := "SELECT COUNT(*) " + baseQuery
	// Ensure all fields from models.Domain are selected, especially is_favorite and updated_at
	selectQuery := "SELECT id, target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite " + baseQuery

	// Apply search filters
	if filters.DomainNameSearch != "" {
		baseQuery += "AND LOWER(domain_name) LIKE LOWER(?) "
		args = append(args, "%"+filters.DomainNameSearch+"%")
		countArgs = append(countArgs, "%"+filters.DomainNameSearch+"%")
	}
	if filters.SourceSearch != "" {
		baseQuery += "AND LOWER(source) LIKE LOWER(?) "
		args = append(args, "%"+filters.SourceSearch+"%")
		countArgs = append(countArgs, "%"+filters.SourceSearch+"%")
	}

	if filters.IsInScope != nil {
		baseQuery += "AND is_in_scope = ? "
		args = append(args, *filters.IsInScope)
		countArgs = append(countArgs, *filters.IsInScope)
	}
	if filters.IsFavorite != nil {
		baseQuery += "AND is_favorite = ? "
		args = append(args, *filters.IsFavorite)
		countArgs = append(countArgs, *filters.IsFavorite)
	}

	// Update countQuery and selectQuery with the filtered baseQuery
	countQuery = "SELECT COUNT(*) " + baseQuery
	selectQuery = "SELECT id, target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite " + baseQuery

	err := DB.QueryRow(countQuery, countArgs...).Scan(&totalRecords)
	if err != nil {
		logger.Error("Error counting domains: %v. Query: %s, Args: %v", err, countQuery, countArgs)
		return nil, 0, fmt.Errorf("counting domains failed: %w", err)
	}

	// Validate sort by column to prevent SQL injection
	allowedSortCols := map[string]bool{
		"id":                true,
		"domain_name":       true,
		"source":            true,
		"is_in_scope":       true,
		"is_wildcard_scope": true,
		"notes":             true,
		"created_at":        true,
		"updated_at":        true,
		"is_favorite":       true,
	}
	if !allowedSortCols[filters.SortBy] {
		filters.SortBy = "domain_name" // Default sort column if invalid
	}
	if filters.SortOrder != "ASC" && filters.SortOrder != "DESC" {
		filters.SortOrder = "ASC" // Default sort order
	}
	orderByClause := fmt.Sprintf("ORDER BY %s %s, id %s", filters.SortBy, filters.SortOrder, filters.SortOrder)
	selectQuery += orderByClause

	if filters.Limit > 0 {
		offset := (filters.Page - 1) * filters.Limit
		selectQuery += " LIMIT ? OFFSET ?"
		args = append(args, filters.Limit, offset)
	}

	rows, err := DB.Query(selectQuery, args...)
	if err != nil {
		logger.Error("Error querying domains: %v. Query: %s, Args: %v", err, selectQuery, args)
		return nil, 0, fmt.Errorf("querying domains failed (query: %s): %w", selectQuery, err)
	}
	defer rows.Close()

	for rows.Next() {
		var d models.Domain
		var createdAtStr string
		var updatedAtStr string
		if err := rows.Scan(&d.ID, &d.TargetID, &d.DomainName, &d.Source, &d.IsInScope, &d.IsWildcardScope, &d.Notes, &createdAtStr, &updatedAtStr, &d.IsFavorite); err != nil {
			logger.Error("Error scanning domain row: %v", err)
			return nil, 0, fmt.Errorf("scanning domain row failed: %w", err)
		}
		// Parse timestamps
		d.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			logger.Error("Error parsing created_at for domain %d: %v", d.ID, err)
		}
		d.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			logger.Error("Error parsing updated_at for domain %d: %v", d.ID, err)
		}
		domains = append(domains, d)
	}

	if err = rows.Err(); err != nil {
		logger.Error("Error iterating domain rows: %v", err)
		return nil, 0, fmt.Errorf("iterating domain rows failed: %w", err)
	}
	return domains, totalRecords, nil
}

// CreateDomain creates a new domain entry in the database.
func CreateDomain(domain models.Domain) (int64, error) {
	if DB == nil {
		return 0, errors.New("database connection is not initialized")
	}

	// Ensure domain_name is not empty
	if strings.TrimSpace(domain.DomainName) == "" {
		return 0, errors.New("domain_name cannot be empty")
	}

	// Check for existing domain for the same target
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
// Currently supports updating source, is_in_scope, and notes.
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

// GetDomainByID retrieves a single domain by its ID.
func GetDomainByID(id int64) (*models.Domain, error) {
	if DB == nil {
		return nil, errors.New("database connection is not initialized")
	}
	var d models.Domain
	var createdAtStr, updatedAtStr string
	err := DB.QueryRow("SELECT id, target_id, domain_name, source, is_in_scope, is_wildcard_scope, notes, created_at, updated_at, is_favorite FROM domains WHERE id = ?", id).Scan(
		&d.ID, &d.TargetID, &d.DomainName, &d.Source, &d.IsInScope, &d.IsWildcardScope, &d.Notes, &createdAtStr, &updatedAtStr, &d.IsFavorite,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("domain with ID %d not found", id)
		}
		logger.Error("Error scanning domain by ID %d: %v", id, err)
		return nil, fmt.Errorf("querying domain by ID failed: %w", err)
	}
	d.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		logger.Error("Error parsing created_at for domain ID %d: %v", id, err)
		// Potentially return error or use zero time
	}
	d.UpdatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		logger.Error("Error parsing updated_at for domain ID %d: %v", id, err)
		// Potentially return error or use zero time
	}
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
