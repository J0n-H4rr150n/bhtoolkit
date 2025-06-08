package database

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"toolkit/logger"
	"toolkit/models"

	"github.com/google/uuid"
)

var validScopeItemTypesDB = map[string]bool{"domain": true, "subdomain": true, "ip_address": true, "cidr": true, "url_path": true}

// slugify creates a URL-friendly slug from a string.
func slugify(s string) string {
	slug := strings.ToLower(s)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return uuid.New().String()[:8] // Fallback to a short UUID if slug becomes empty
	}
	return slug
}

// determineItemType infers item_type from pattern for scope rules.
func determineItemType(pattern string) string {
	if strings.HasPrefix(pattern, "/") {
		return "url_path"
	}
	if regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(/\d{1,2})?$`).MatchString(pattern) {
		if strings.Contains(pattern, "/") {
			return "cidr"
		}
		return "ip_address"
	}
	// Basic check for domain/subdomain. More robust validation might be needed.
	if strings.Contains(pattern, ".") && !strings.ContainsAny(pattern, " /") {
		if strings.HasPrefix(pattern, "*.") {
			return "subdomain" // Or treat as domain with wildcard
		}
		return "domain"
	}
	return "domain" // Default or consider error/unknown
}

// CreateTargetWithScopeRules creates a new target and its associated scope rules within a transaction.
func CreateTargetWithScopeRules(targetData models.TargetCreateRequest) (models.Target, error) {
	var createdTarget models.Target

	// Validate platform existence
	var platformExists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM platforms WHERE id = ?)", targetData.PlatformID).Scan(&platformExists)
	if err != nil {
		return createdTarget, fmt.Errorf("error checking platform existence for PlatformID %d: %w", targetData.PlatformID, err)
	}
	if !platformExists {
		return createdTarget, fmt.Errorf("platform with ID %d does not exist", targetData.PlatformID)
	}

	// Check for existing target codename
	var existingTargetID int64
	err = DB.QueryRow("SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)", targetData.PlatformID, targetData.Codename).Scan(&existingTargetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return createdTarget, fmt.Errorf("error checking for existing target codename '%s': %w", targetData.Codename, err)
	}
	if err == nil {
		return createdTarget, fmt.Errorf("target with codename '%s' already exists for this platform", targetData.Codename)
	}

	generatedSlug := slugify(targetData.Codename)
	var existingSlugID int64
	err = DB.QueryRow("SELECT id FROM targets WHERE slug = ?", generatedSlug).Scan(&existingSlugID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return createdTarget, fmt.Errorf("error checking for existing slug '%s': %w", generatedSlug, err)
	}
	if err == nil { // Slug conflict
		generatedSlug = fmt.Sprintf("%s-%s", generatedSlug, uuid.New().String()[:4])
	}

	tx, err := DB.Begin()
	if err != nil {
		return createdTarget, fmt.Errorf("beginning database transaction: %w", err)
	}

	targetStmt, err := tx.Prepare(`INSERT INTO targets (platform_id, slug, codename, link, notes) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return createdTarget, fmt.Errorf("preparing target insert statement: %w", err)
	}
	defer targetStmt.Close()

	res, err := targetStmt.Exec(targetData.PlatformID, generatedSlug, targetData.Codename, targetData.Link, targetData.Notes)
	if err != nil {
		tx.Rollback()
		return createdTarget, fmt.Errorf("executing target insert statement: %w", err)
	}

	targetID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		return createdTarget, fmt.Errorf("getting last insert ID for target: %w", err)
	}

	createdTarget = models.Target{
		ID: targetID, PlatformID: targetData.PlatformID, Slug: generatedSlug,
		Codename: targetData.Codename, Link: targetData.Link, Notes: targetData.Notes,
	}

	scopeStmt, err := tx.Prepare(`INSERT INTO scope_rules (target_id, item_type, pattern, is_in_scope, is_wildcard, description) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return createdTarget, fmt.Errorf("preparing scope_rules insert statement: %w", err)
	}
	defer scopeStmt.Close()

	allScopeItems := append(targetData.InScopeItems, targetData.OutOfScopeItems...)
	isInScopeFlag := true
	createdScopeRules := []models.ScopeRule{}

	for i, item := range allScopeItems {
		if i >= len(targetData.InScopeItems) {
			isInScopeFlag = false
		}
		itemType := strings.ToLower(strings.TrimSpace(item.ItemType))
		if itemType == "" {
			itemType = determineItemType(item.Pattern)
		} else if !validScopeItemTypesDB[itemType] {
			tx.Rollback()
			return createdTarget, fmt.Errorf("invalid item_type '%s' for pattern '%s'", item.ItemType, item.Pattern)
		}
		isWildcard := strings.Contains(item.Pattern, "*")
		scopeRes, err := scopeStmt.Exec(targetID, itemType, item.Pattern, isInScopeFlag, isWildcard, item.Description)
		if err != nil {
			tx.Rollback()
			return createdTarget, fmt.Errorf("inserting scope rule '%s': %w", item.Pattern, err)
		}
		scopeID, _ := scopeRes.LastInsertId()
		createdScopeRules = append(createdScopeRules, models.ScopeRule{ID: scopeID, TargetID: targetID, ItemType: itemType, Pattern: item.Pattern, IsInScope: isInScopeFlag, IsWildcard: isWildcard, Description: item.Description})
	}

	if err := tx.Commit(); err != nil {
		return createdTarget, fmt.Errorf("committing transaction: %w", err)
	}
	createdTarget.ScopeRules = createdScopeRules
	return createdTarget, nil
}

// GetTargets retrieves targets, optionally filtered by platform ID.
func GetTargets(platformIDFilter *int64) ([]models.Target, error) {
	query := "SELECT id, platform_id, slug, codename, link, notes FROM targets"
	args := []interface{}{}

	if platformIDFilter != nil {
		query += " WHERE platform_id = ?"
		args = append(args, *platformIDFilter)
	}
	query += " ORDER BY codename ASC"

	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying targets: %w", err)
	}
	defer rows.Close()

	var targets []models.Target
	for rows.Next() {
		var t models.Target
		var slug, notes sql.NullString
		if err := rows.Scan(&t.ID, &t.PlatformID, &slug, &t.Codename, &t.Link, &notes); err != nil {
			return nil, fmt.Errorf("scanning target row: %w", err)
		}
		t.Slug = slug.String
		t.Notes = notes.String
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// GetTargetByID retrieves a single target by its ID, including its scope rules.
func GetTargetByID(targetID int64) (models.Target, error) {
	var t models.Target
	var slug, notes sql.NullString
	err := DB.QueryRow(`SELECT id, platform_id, slug, codename, link, notes FROM targets WHERE id = ?`, targetID).Scan(
		&t.ID, &t.PlatformID, &slug, &t.Codename, &t.Link, &notes,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return t, fmt.Errorf("target with ID %d not found", targetID)
		}
		return t, fmt.Errorf("querying target ID %d: %w", targetID, err)
	}
	t.Slug = slug.String
	t.Notes = notes.String

	t.ScopeRules, err = GetAllScopeRulesForTarget(targetID) // Assumes GetAllScopeRulesForTarget is in this package or imported
	if err != nil {
		// Log the error but still return the target details found so far
		logger.Error("GetTargetByID: Error fetching scope rules for target %d: %v", targetID, err)
	}
	return t, nil
}

// UpdateTargetDetails updates the link and notes for a target.
func UpdateTargetDetails(targetID int64, link, notes string) error {
	stmt, err := DB.Prepare("UPDATE targets SET link = ?, notes = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("preparing update target statement for ID %d: %w", targetID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(link, notes, targetID)
	if err != nil {
		return fmt.Errorf("executing update target statement for ID %d: %w", targetID, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("target with ID %d not found for update", targetID)
	}
	return nil
}

// DeleteTargetByIDOrSlug deletes a target by its ID or slug.
func DeleteTargetByIDOrSlug(identifier string) (bool, error) {
	var query string
	var argToUse interface{}

	targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
	if parseErr == nil {
		query = "DELETE FROM targets WHERE id = ?"
		argToUse = targetID // Use the parsed int64 for the query argument
	} else {
		query = "DELETE FROM targets WHERE slug = ?"
		argToUse = identifier // Use the original string (slug) for the query argument
	}

	stmt, err := DB.Prepare(query)
	if err != nil {
		return false, fmt.Errorf("preparing delete target statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(argToUse) // Use the correctly typed argument
	if err != nil {
		return false, fmt.Errorf("executing delete target statement: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	return rowsAffected > 0, nil
}

// DeleteTargetByCodenameAndPlatform deletes a target by its codename and platform ID.
func DeleteTargetByCodenameAndPlatform(platformID int64, codename string) (bool, error) {
	stmt, err := DB.Prepare("DELETE FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)")
	if err != nil {
		return false, fmt.Errorf("preparing delete target by codename statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(platformID, codename)
	if err != nil {
		return false, fmt.Errorf("executing delete target by codename statement: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	return rowsAffected > 0, nil
}
