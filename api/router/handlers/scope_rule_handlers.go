package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// ScopeRulesHandler handles requests for the /api/scope-rules path (collection operations).
// It dispatches to addScopeRule for POST and getScopeRules for GET.
func ScopeRulesHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("ScopeRulesHandler received request: Method %s, Relative Path %s", r.Method, r.URL.Path)
	switch r.Method {
	case http.MethodPost:
		addScopeRule(w, r)
	case http.MethodGet:
		getScopeRules(w, r)
	default:
		logger.Error("ScopeRulesHandler: MethodNotAllowed: %s for /scope-rules", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed for /scope-rules collection"})
	}
}

// ScopeRuleItemHandler handles requests for a specific scope rule item, e.g., /api/scope-rules/{id}
func ScopeRuleItemHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	var idStr string
	// Path should be /scope-rules/{id}
	// After trimming and splitting, parts should be ["scope-rules", "{id}"]
	if len(parts) == 2 && parts[0] == "scope-rules" {
		idStr = parts[1]
	} else {
		logger.Error("ScopeRuleItemHandler: Malformed path for scope rule item: %s. Expected /scope-rules/{id}", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	ruleID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Error("ScopeRuleItemHandler: Invalid scope rule ID '%s' in path: %v", idStr, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid scope rule ID format. Must be numeric."})
		return
	}

	logger.Info("ScopeRuleItemHandler received request: Method %s, Relative Path %s, Parsed RuleID %d", r.Method, r.URL.Path, ruleID)

	switch r.Method {
	case http.MethodGet:
		getScopeRuleByID(w, r, ruleID)
	// case http.MethodPut:
	// 	updateScopeRule(w, r, ruleID) // TODO: Implement update
	case http.MethodDelete:
		deleteScopeRule(w, r, ruleID)
	default:
		logger.Error("ScopeRuleItemHandler: MethodNotAllowed: %s for /scope-rules/%d", r.Method, ruleID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed for this scope rule resource"})
	}
}

// addScopeRule handles adding a new scope rule to a target.
func addScopeRule(w http.ResponseWriter, r *http.Request) {
	var sr models.ScopeRule
	if err := json.NewDecoder(r.Body).Decode(&sr); err != nil {
		logger.Error("addScopeRule: Error decoding request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
		return
	}
	defer r.Body.Close()

	if sr.TargetID == 0 {
		logger.Error("addScopeRule: target_id is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "target_id is required"})
		return
	}
	sr.Pattern = strings.TrimSpace(sr.Pattern)
	if sr.Pattern == "" {
		logger.Error("addScopeRule: pattern is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "pattern is required"})
		return
	}
	sr.ItemType = strings.TrimSpace(strings.ToLower(sr.ItemType))
	validItemTypes := map[string]bool{"domain": true, "subdomain": true, "ip_address": true, "cidr": true, "url_path": true}
	if !validItemTypes[sr.ItemType] {
		logger.Error("addScopeRule: item_type is required and was empty or invalid: %s", sr.ItemType)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "item_type is required and must be valid (domain, subdomain, ip_address, cidr, url_path)"})
		return
	}

	var targetExists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", sr.TargetID).Scan(&targetExists)
	if err != nil {
		logger.Error("addScopeRule: Error checking target existence for TargetID %d: %v", sr.TargetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while verifying target"})
		return
	}
	if !targetExists {
		logger.Error("addScopeRule: TargetID %d does not exist", sr.TargetID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target with ID %d does not exist", sr.TargetID)})
		return
	}

	var existingRuleID int64
	err = database.DB.QueryRow(`SELECT id FROM scope_rules 
                                WHERE target_id = ? AND item_type = ? AND pattern = ? AND is_in_scope = ?`,
		sr.TargetID, sr.ItemType, sr.Pattern, sr.IsInScope).Scan(&existingRuleID)

	if err != nil && err != sql.ErrNoRows {
		logger.Error("addScopeRule: Error checking for existing scope rule for target %d, pattern '%s': %v", sr.TargetID, sr.Pattern, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while checking for duplicate scope rule"})
		return
	}
	if err == nil {
		logger.Error("addScopeRule: Duplicate scope rule for target %d, pattern '%s', type '%s', in_scope '%t'. Existing rule ID: %d", sr.TargetID, sr.Pattern, sr.ItemType, sr.IsInScope, existingRuleID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "This exact scope rule already exists for this target."})
		return
	}

	sr.IsWildcard = strings.Contains(sr.Pattern, "*")

	stmt, err := database.DB.Prepare(`INSERT INTO scope_rules 
        (target_id, item_type, pattern, is_in_scope, is_wildcard, description) 
        VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		logger.Error("addScopeRule: Error preparing statement for scope rule on target %d: %v", sr.TargetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(sr.TargetID, sr.ItemType, sr.Pattern, sr.IsInScope, sr.IsWildcard, sr.Description)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			logger.Error("addScopeRule: Database UNIQUE constraint failed for target %d, pattern '%s', type '%s', in_scope '%t'. This indicates a duplicate.", sr.TargetID, sr.Pattern, sr.ItemType, sr.IsInScope)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "This exact scope rule already exists for this target (DB constraint)."})
		} else {
			logger.Error("addScopeRule: Error inserting scope rule for target %d, pattern '%s': %v", sr.TargetID, sr.Pattern, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while inserting scope rule"})
		}
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		logger.Error("addScopeRule: Error getting last insert ID for scope rule on target %d: %v", sr.TargetID, err)
	}
	sr.ID = id

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(sr); err != nil {
		logger.Error("addScopeRule: Error encoding response for scope rule on target %d: %v", sr.TargetID, err)
	}
	logger.Info("Scope rule created: ID %d, TargetID %d, Pattern '%s', InScope: %t", sr.ID, sr.TargetID, sr.Pattern, sr.IsInScope)
}

// getScopeRules handles listing scope rules for a target.
func getScopeRules(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	if targetIDStr == "" {
		logger.Error("getScopeRules: target_id query parameter is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "target_id query parameter is required"})
		return
	}

	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("getScopeRules: Invalid target_id parameter '%s': %v", targetIDStr, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid target_id parameter, must be an integer"})
		return
	}

	var targetExists bool
	err = database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", targetID).Scan(&targetExists)
	if err != nil {
		logger.Error("getScopeRules: Error checking target existence for TargetID %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while verifying target"})
		return
	}
	if !targetExists {
		logger.Error("getScopeRules: TargetID %d does not exist when fetching scope rules", targetID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target with ID %d does not exist", targetID)})
		return
	}

	rows, err := database.DB.Query(`SELECT id, target_id, item_type, pattern, is_in_scope, is_wildcard, description 
                                    FROM scope_rules WHERE target_id = ? ORDER BY id ASC`, targetID)
	if err != nil {
		logger.Error("getScopeRules: Error querying scope rules for target_id %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer rows.Close()

	scopeRules := []models.ScopeRule{}
	for rows.Next() {
		var sr models.ScopeRule
		if err := rows.Scan(&sr.ID, &sr.TargetID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description); err != nil {
			logger.Error("getScopeRules: Error scanning scope rule row for target_id %d: %v", targetID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
			return
		}
		scopeRules = append(scopeRules, sr)
	}
	if err = rows.Err(); err != nil {
		logger.Error("getScopeRules: Error iterating scope rule rows for target_id %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scopeRules); err != nil {
		logger.Error("getScopeRules: Error encoding response for target_id %d: %v", targetID, err)
	}
	logger.Info("Fetched %d scope rules for target_id %d", len(scopeRules), targetID)
}

// getScopeRuleByID handles fetching a single scope rule by its ID.
func getScopeRuleByID(w http.ResponseWriter, r *http.Request, ruleID int64) {
	var sr models.ScopeRule
	err := database.DB.QueryRow(`SELECT id, target_id, item_type, pattern, is_in_scope, is_wildcard, description 
                                 FROM scope_rules WHERE id = ?`, ruleID).Scan(
		&sr.ID, &sr.TargetID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			logger.Error("getScopeRuleByID: Scope rule with ID %d not found", ruleID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Scope rule with ID %d not found", ruleID)})
		} else {
			logger.Error("getScopeRuleByID: Error querying scope rule ID %d: %v", ruleID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sr); err != nil {
		logger.Error("getScopeRuleByID: Error encoding response for scope rule ID %d: %v", ruleID, err)
	}
	logger.Info("Fetched scope rule by ID: %d", sr.ID)
}

// deleteScopeRule handles deleting a specific scope rule by its ID.
func deleteScopeRule(w http.ResponseWriter, r *http.Request, ruleID int64) {
	logger.Info("Attempting to delete scope rule with ID %d", ruleID)

	stmt, err := database.DB.Prepare("DELETE FROM scope_rules WHERE id = ?")
	if err != nil {
		logger.Error("deleteScopeRule: Error preparing delete statement for rule ID %d: %v", ruleID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(ruleID)
	if err != nil {
		logger.Error("deleteScopeRule: Error executing delete for rule ID %d: %v", ruleID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("deleteScopeRule: Error getting rows affected for rule ID %d: %v", ruleID, err)
		w.WriteHeader(http.StatusNoContent) // Still proceed with 204 if exec was successful but rowsAffected failed
		return
	}

	if rowsAffected == 0 {
		logger.Error("deleteScopeRule: Scope rule with ID %d not found for deletion", ruleID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Scope rule with ID %d not found", ruleID)})
		return
	}

	logger.Info("Scope rule deleted successfully: ID %d", ruleID)
	w.WriteHeader(http.StatusNoContent)
}
