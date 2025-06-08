package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

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

	createdRule, err := database.AddScopeRule(sr)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			logger.Error("addScopeRule: Conflict: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: err.Error()})
		} else if strings.Contains(err.Error(), "does not exist") { // Target does not exist
			logger.Error("addScopeRule: Bad Request: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest) // Or StatusNotFound depending on preference
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: err.Error()})
		} else {
			logger.Error("addScopeRule: Internal server error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdRule); err != nil {
		logger.Error("addScopeRule: Error encoding response for scope rule on target %d: %v", createdRule.TargetID, err)
	}
	logger.Info("Scope rule created: ID %d, TargetID %d, Pattern '%s', InScope: %t", createdRule.ID, createdRule.TargetID, createdRule.Pattern, createdRule.IsInScope)
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

	// Optional: Validate target existence here if not done by database.GetScopeRulesByTargetID
	// For now, assume database function might return empty if target doesn't exist or error.

	scopeRules, err := database.GetScopeRulesByTargetID(targetID)
	if err != nil {
		logger.Error("getScopeRules: Error querying scope rules for target_id %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	if scopeRules == nil { // Ensure an empty array is returned instead of null
		scopeRules = []models.ScopeRule{}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scopeRules); err != nil {
		logger.Error("getScopeRules: Error encoding response for target_id %d: %v", targetID, err)
	}
	logger.Info("Fetched %d scope rules for target_id %d", len(scopeRules), targetID)
}

// GetScopeRuleByIDChiHandler is the chi-compatible handler for getting a scope rule by ID.
func GetScopeRuleByIDChiHandler(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := chi.URLParam(r, "ruleID")
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		logger.Error("GetScopeRuleByIDChiHandler: Invalid rule ID format '%s': %v", ruleIDStr, err)
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}
	getScopeRuleByID(w, r, ruleID)
}

// getScopeRuleByID handles fetching a single scope rule by its ID.
func getScopeRuleByID(w http.ResponseWriter, r *http.Request, ruleID int64) {
	sr, err := database.GetScopeRuleByID(ruleID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
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

// DeleteScopeRuleChiHandler is the chi-compatible handler for deleting a scope rule.
func DeleteScopeRuleChiHandler(w http.ResponseWriter, r *http.Request) {
	ruleIDStr := chi.URLParam(r, "ruleID")
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		logger.Error("DeleteScopeRuleChiHandler: Invalid rule ID format '%s': %v", ruleIDStr, err)
		http.Error(w, "Invalid rule ID", http.StatusBadRequest)
		return
	}
	deleteScopeRule(w, r, ruleID)
}

// deleteScopeRule handles deleting a specific scope rule by its ID.
func deleteScopeRule(w http.ResponseWriter, r *http.Request, ruleID int64) {
	logger.Info("Attempting to delete scope rule with ID %d", ruleID)

	err := database.DeleteScopeRule(ruleID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("deleteScopeRule: Scope rule with ID %d not found for deletion", ruleID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Scope rule with ID %d not found", ruleID)})
		} else {
			logger.Error("deleteScopeRule: Error deleting rule ID %d: %v", ruleID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		}
		return
	}

	logger.Info("Scope rule deleted successfully: ID %d", ruleID)
	w.WriteHeader(http.StatusNoContent)
}
