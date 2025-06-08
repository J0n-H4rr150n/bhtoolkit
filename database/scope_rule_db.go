package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"toolkit/logger"
	"toolkit/models"
)

// AddScopeRule inserts a new scope rule into the database.
func AddScopeRule(rule models.ScopeRule) (models.ScopeRule, error) {
	// Validate target existence
	var targetExists bool
	err := DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", rule.TargetID).Scan(&targetExists)
	if err != nil {
		return rule, fmt.Errorf("error checking target existence for TargetID %d: %w", rule.TargetID, err)
	}
	if !targetExists {
		return rule, fmt.Errorf("target with ID %d does not exist", rule.TargetID)
	}

	// Check for existing rule (duplicate)
	var existingRuleID int64
	err = DB.QueryRow(`SELECT id FROM scope_rules 
	                   WHERE target_id = ? AND item_type = ? AND pattern = ? AND is_in_scope = ?`,
		rule.TargetID, rule.ItemType, rule.Pattern, rule.IsInScope).Scan(&existingRuleID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return rule, fmt.Errorf("error checking for existing scope rule for target %d, pattern '%s': %w", rule.TargetID, rule.Pattern, err)
	}
	if err == nil { // Rule already exists
		return rule, fmt.Errorf("this exact scope rule already exists for this target (ID: %d)", existingRuleID)
	}

	rule.IsWildcard = strings.Contains(rule.Pattern, "*")

	stmt, err := DB.Prepare(`INSERT INTO scope_rules 
        (target_id, item_type, pattern, is_in_scope, is_wildcard, description) 
        VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return rule, fmt.Errorf("preparing insert scope rule statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(rule.TargetID, rule.ItemType, rule.Pattern, rule.IsInScope, rule.IsWildcard, rule.Description)
	if err != nil {
		// The UNIQUE constraint in the DB should catch duplicates if the above check somehow misses.
		return rule, fmt.Errorf("executing insert scope rule statement: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return rule, fmt.Errorf("getting last insert ID for scope rule: %w", err)
	}
	rule.ID = id
	return rule, nil
}

// GetScopeRulesByTargetID retrieves all scope rules for a given target ID.
func GetScopeRulesByTargetID(targetID int64) ([]models.ScopeRule, error) {
	rows, err := DB.Query(`SELECT id, target_id, item_type, pattern, is_in_scope, is_wildcard, description 
                           FROM scope_rules WHERE target_id = ? ORDER BY id ASC`, targetID)
	if err != nil {
		return nil, fmt.Errorf("querying scope rules for target_id %d: %w", targetID, err)
	}
	defer rows.Close()

	var rules []models.ScopeRule
	for rows.Next() {
		var sr models.ScopeRule
		if err := rows.Scan(&sr.ID, &sr.TargetID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description); err != nil {
			return nil, fmt.Errorf("scanning scope rule row for target_id %d: %w", targetID, err)
		}
		rules = append(rules, sr)
	}
	return rules, rows.Err()
}

// GetScopeRuleByID retrieves a single scope rule by its ID.
func GetScopeRuleByID(ruleID int64) (models.ScopeRule, error) {
	var sr models.ScopeRule
	err := DB.QueryRow(`SELECT id, target_id, item_type, pattern, is_in_scope, is_wildcard, description 
                         FROM scope_rules WHERE id = ?`, ruleID).Scan(
		&sr.ID, &sr.TargetID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sr, fmt.Errorf("scope rule with ID %d not found", ruleID)
		}
		return sr, fmt.Errorf("querying scope rule ID %d: %w", ruleID, err)
	}
	return sr, nil
}

// DeleteScopeRule deletes a scope rule by its ID.
func DeleteScopeRule(ruleID int64) error {
	stmt, err := DB.Prepare("DELETE FROM scope_rules WHERE id = ?")
	if err != nil {
		return fmt.Errorf("preparing delete scope rule statement for ID %d: %w", ruleID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(ruleID)
	if err != nil {
		return fmt.Errorf("executing delete scope rule statement for ID %d: %w", ruleID, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("scope rule with ID %d not found for deletion", ruleID)
	}
	return nil
}

// GetInScopeRulesForTarget is now part of this file.
func GetInScopeRulesForTarget(targetID int64) ([]models.ScopeRule, error) {
	// This function is identical to GetScopeRulesByTargetID but filters for is_in_scope = TRUE
	// For simplicity, we can call GetScopeRulesByTargetID and filter in Go, or use a more specific query.
	// Using a specific query is generally more efficient.
	rows, err := DB.Query(`SELECT id, item_type, pattern, is_in_scope, is_wildcard, description 
                           FROM scope_rules 
                           WHERE target_id = ? AND is_in_scope = TRUE 
                           ORDER BY id ASC`, targetID)
	if err != nil {
		return nil, fmt.Errorf("querying in-scope rules for target %d: %w", targetID, err)
	}
	defer rows.Close()

	var rules []models.ScopeRule
	for rows.Next() {
		var sr models.ScopeRule
		sr.TargetID = targetID // Set manually as it's not selected in this query but known
		if err := rows.Scan(&sr.ID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description); err != nil {
			return nil, fmt.Errorf("scanning in-scope rule for target %d: %w", targetID, err)
		}
		rules = append(rules, sr)
	}
	return rules, rows.Err()
}

// GetAllScopeRulesForTarget is now part of this file.
// It's an alias for GetScopeRulesByTargetID for clarity if used elsewhere.
func GetAllScopeRulesForTarget(targetID int64) ([]models.ScopeRule, error) {
	logger.Debug("GetAllScopeRulesForTarget called for targetID: %d, delegating to GetScopeRulesByTargetID", targetID)
	return GetScopeRulesByTargetID(targetID)
}
