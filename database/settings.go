package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"toolkit/logger"
	"toolkit/models"
)

// GetSetting retrieves a specific setting value from the app_settings table.
func GetSetting(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // Return empty string if not found, not an error
		}
		return "", fmt.Errorf("failed to get setting '%s': %w", key, err)
	}
	return value, nil
}

// SetSetting saves or updates a specific setting value in the app_settings table.
func SetSetting(key, value string) error {
	stmt, err := DB.Prepare("INSERT OR REPLACE INTO app_settings (key, value) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare set setting statement for key '%s': %w", key, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(key, value)
	if err != nil {
		return fmt.Errorf("failed to execute set setting for key '%s': %w", key, err)
	}
	return nil
}

// GetProxyExclusionRules retrieves the list of global proxy exclusion rules.
func GetProxyExclusionRules() ([]models.ProxyExclusionRule, error) {
	rulesJSON, err := GetSetting(models.ProxyExclusionRulesKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get proxy exclusion rules setting: %w", err)
	}

	var rules []models.ProxyExclusionRule
	if rulesJSON == "" {
		// No rules set yet, return an empty slice
		return []models.ProxyExclusionRule{}, nil
	}

	if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
		logger.Error("GetProxyExclusionRules: Error unmarshalling rules JSON: %v. Stored value: %s", err, rulesJSON)
		return nil, fmt.Errorf("failed to unmarshal proxy exclusion rules: %w", err)
	}
	return rules, nil
}

// SetProxyExclusionRules saves the list of global proxy exclusion rules.
func SetProxyExclusionRules(rules []models.ProxyExclusionRule) error {
	if rules == nil {
		// If nil is passed, store an empty JSON array to represent no rules.
		rules = []models.ProxyExclusionRule{}
	}

	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("failed to marshal proxy exclusion rules to JSON: %w", err)
	}

	if err := SetSetting(models.ProxyExclusionRulesKey, string(rulesJSON)); err != nil {
		return fmt.Errorf("failed to save proxy exclusion rules setting: %w", err)
	}
	return nil
}
