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
	var findingReq models.TargetFinding // Use TargetFinding directly, ID will be ignored by DB on insert
	if err := json.NewDecoder(r.Body).Decode(&findingReq); err != nil {
		logger.Error("CreateTargetFindingHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if findingReq.TargetID == 0 {
		logger.Error("CreateTargetFindingHandler: target_id is required in the request body")
		http.Error(w, "target_id is required in the request body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(findingReq.Title) == "" {
		logger.Error("CreateTargetFindingHandler: Finding title cannot be empty for target_id %d", findingReq.TargetID)
		http.Error(w, "Finding title cannot be empty", http.StatusBadRequest)
		return
	}

	id, err := database.CreateTargetFinding(findingReq)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Error creating finding for target %d: %v", findingReq.TargetID, err)
		http.Error(w, "Failed to create finding", http.StatusInternalServerError)
		return
	}

	createdFinding, err := database.GetTargetFindingByID(id)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Error fetching newly created finding %d: %v", id, err)
		// The finding was created, so return 201 with the ID.
		// Provide a clear message about the situation.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated) // Respond with 201 Created
		json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "target_id": findingReq.TargetID, "message": "Finding created successfully. However, there was an error retrieving the full details of the created finding."})
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

// GetFindingByIDHandler handles GET requests for a single finding by its ID.
func GetFindingByIDHandler(w http.ResponseWriter, r *http.Request) {
	findingIDStr := chi.URLParam(r, "finding_id")
	findingID, err := strconv.ParseInt(findingIDStr, 10, 64)
	if err != nil {
		logger.Error("GetFindingByIDHandler: Invalid finding_id format: %s", findingIDStr)
		http.Error(w, "Invalid finding ID format", http.StatusBadRequest)
		return
	}

	finding, err := database.GetTargetFindingByID(findingID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("GetFindingByIDHandler: Finding %d not found: %v", findingID, err)
			http.Error(w, "Finding not found", http.StatusNotFound)
		} else {
			logger.Error("GetFindingByIDHandler: Error fetching finding %d: %v", findingID, err)
			http.Error(w, "Failed to retrieve finding", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finding)
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
		// The finding was updated, so return 200.
		// Provide a clear message.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": findingID, "target_id": existingFinding.TargetID, "message": "Finding updated successfully. However, there was an error retrieving the full details of the updated finding."})
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
