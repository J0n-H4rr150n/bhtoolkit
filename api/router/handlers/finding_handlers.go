package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// CreateTargetFindingHandler handles POST requests to create a new finding for a target.
func CreateTargetFindingHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Invalid target_id format: %s", targetIDStr)
		http.Error(w, "Invalid target ID format", http.StatusBadRequest)
		return
	}

	var findingReq models.TargetFinding // Use TargetFinding directly, ID will be ignored by DB on insert
	if err := json.NewDecoder(r.Body).Decode(&findingReq); err != nil {
		logger.Error("CreateTargetFindingHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(findingReq.Title) == "" {
		http.Error(w, "Finding title cannot be empty", http.StatusBadRequest)
		return
	}

	findingReq.TargetID = targetID // Ensure target_id from URL is used

	id, err := database.CreateTargetFinding(findingReq)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Error creating finding for target %d: %v", targetID, err)
		http.Error(w, "Failed to create finding", http.StatusInternalServerError)
		return
	}

	createdFinding, err := database.GetTargetFindingByID(id)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Error fetching newly created finding %d: %v", id, err)
		// Still return 201, but log the issue
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "message": "Finding created, but error fetching full details."})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdFinding)
}

// GetTargetFindingsHandler handles GET requests to list findings for a target.
func GetTargetFindingsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("GetTargetFindingsHandler: Invalid target_id format: %s", targetIDStr)
		http.Error(w, "Invalid target ID format", http.StatusBadRequest)
		return
	}

	findings, err := database.GetTargetFindingsByTargetID(targetID)
	if err != nil {
		logger.Error("GetTargetFindingsHandler: Error fetching findings for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve findings", http.StatusInternalServerError)
		return
	}

	if findings == nil { // Ensure we return an empty array, not null
		findings = []models.TargetFinding{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(findings)
}

// UpdateTargetFindingHandler handles PUT requests to update an existing finding.
func UpdateTargetFindingHandler(w http.ResponseWriter, r *http.Request) {
	findingIDStr := chi.URLParam(r, "finding_id")
	findingID, err := strconv.ParseInt(findingIDStr, 10, 64)
	if err != nil {
		logger.Error("UpdateTargetFindingHandler: Invalid finding_id format: %s", findingIDStr)
		http.Error(w, "Invalid finding ID format", http.StatusBadRequest)
		return
	}

	var findingUpdateReq models.TargetFinding
	if err := json.NewDecoder(r.Body).Decode(&findingUpdateReq); err != nil {
		logger.Error("UpdateTargetFindingHandler: Error decoding request body for finding %d: %v", findingID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(findingUpdateReq.Title) == "" {
		http.Error(w, "Finding title cannot be empty", http.StatusBadRequest)
		return
	}

	// Fetch existing to get target_id and ensure it exists
	existingFinding, err := database.GetTargetFindingByID(findingID)
	if err != nil {
		logger.Error("UpdateTargetFindingHandler: Finding %d not found: %v", findingID, err)
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	findingUpdateReq.ID = findingID
	findingUpdateReq.TargetID = existingFinding.TargetID // Preserve original target_id

	if err := database.UpdateTargetFinding(findingUpdateReq); err != nil {
		logger.Error("UpdateTargetFindingHandler: Error updating finding %d: %v", findingID, err)
		http.Error(w, "Failed to update finding", http.StatusInternalServerError)
		return
	}

	updatedFinding, err := database.GetTargetFindingByID(findingID)
	if err != nil {
		logger.Error("UpdateTargetFindingHandler: Error fetching updated finding %d: %v", findingID, err)
		// Still return 200, but log the issue
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Finding updated, but error fetching full details."})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedFinding)
}

// DeleteTargetFindingHandler handles DELETE requests to delete a finding.
func DeleteTargetFindingHandler(w http.ResponseWriter, r *http.Request) {
	findingIDStr := chi.URLParam(r, "finding_id")
	findingID, err := strconv.ParseInt(findingIDStr, 10, 64)
	if err != nil {
		logger.Error("DeleteTargetFindingHandler: Invalid finding_id format: %s", findingIDStr)
		http.Error(w, "Invalid finding ID format", http.StatusBadRequest)
		return
	}

	// To ensure integrity, we might want to pass target_id from context if available,
	// or fetch the finding first to get its target_id.
	// For simplicity here, we assume the frontend will only allow deleting findings
	// for the currently viewed/active target. The DB function can take target_id.
	// This requires knowing the target_id. Let's assume it's passed or fetched.
	// For now, we'll make the DB function require it.
	// We need a way to get target_id here. If it's part of a nested route like /targets/{target_id}/findings/{finding_id}
	// then we can get target_id from chi.URLParam(r, "target_id")
	// If not, we might need to fetch the finding first to get its target_id.

	// Let's assume the route will be /api/findings/{finding_id} and we need to fetch target_id
	finding, err := database.GetTargetFindingByID(findingID)
	if err != nil {
		logger.Error("DeleteTargetFindingHandler: Finding %d not found for deletion: %v", findingID, err)
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	if err := database.DeleteTargetFinding(findingID, finding.TargetID); err != nil {
		logger.Error("DeleteTargetFindingHandler: Error deleting finding %d: %v", findingID, err)
		http.Error(w, "Failed to delete finding", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
