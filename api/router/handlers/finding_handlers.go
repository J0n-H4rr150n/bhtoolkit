package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// VulnerabilityTypeRequest defines the expected JSON payload for creating or updating a vulnerability type.
type VulnerabilityTypeRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateTargetFindingHandler handles POST requests to create a new finding for a target.
func CreateTargetFindingHandler(w http.ResponseWriter, r *http.Request) {
	var findingReq models.TargetFinding
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

	// The findingReq already includes all new fields like Summary, StepsToReproduce, etc.
	// The database.CreateTargetFinding function is expected to handle these.
	id, err := database.CreateTargetFinding(findingReq)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Error creating finding for target %d: %v", findingReq.TargetID, err)
		http.Error(w, "Failed to create finding", http.StatusInternalServerError)
		return
	}

	createdFinding, err := database.GetTargetFindingByID(id)
	if err != nil {
		logger.Error("CreateTargetFindingHandler: Error fetching newly created finding %d: %v", id, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
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

	// database.GetTargetFindingsByTargetID is expected to return all fields, including new ones.
	findings, err := database.GetTargetFindingsByTargetID(targetID)
	if err != nil {
		logger.Error("GetTargetFindingsHandler: Error fetching findings for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve findings", http.StatusInternalServerError)
		return
	}

	if findings == nil {
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

	// database.GetTargetFindingByID is expected to return all fields.
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

	existingFinding, err := database.GetTargetFindingByID(findingID)
	if err != nil {
		logger.Error("UpdateTargetFindingHandler: Finding %d not found: %v", findingID, err)
		http.Error(w, "Finding not found", http.StatusNotFound)
		return
	}

	findingUpdateReq.ID = findingID
	findingUpdateReq.TargetID = existingFinding.TargetID
	findingUpdateReq.DiscoveredAt = existingFinding.DiscoveredAt // Preserve original discovery date

	// database.UpdateTargetFinding is expected to handle all new fields in findingUpdateReq.
	if err := database.UpdateTargetFinding(findingUpdateReq); err != nil {
		logger.Error("UpdateTargetFindingHandler: Error updating finding %d: %v", findingID, err)
		http.Error(w, "Failed to update finding", http.StatusInternalServerError)
		return
	}

	updatedFinding, err := database.GetTargetFindingByID(findingID)
	if err != nil {
		logger.Error("UpdateTargetFindingHandler: Error fetching updated finding %d: %v", findingID, err)
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

// --- VulnerabilityType Handlers ---

// CreateVulnerabilityTypeHandler handles POST requests to create a new vulnerability type.
func CreateVulnerabilityTypeHandler(w http.ResponseWriter, r *http.Request) {
	var req VulnerabilityTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("CreateVulnerabilityTypeHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(req.Name) == "" {
		logger.Error("CreateVulnerabilityTypeHandler: Vulnerability type name cannot be empty")
		http.Error(w, "Vulnerability type name cannot be empty", http.StatusBadRequest)
		return
	}

	vt := models.VulnerabilityType{
		Name:        req.Name,
		Description: models.NullString(req.Description),
	}

	id, err := database.CreateVulnerabilityType(vt)
	if err != nil {
		logger.Error("CreateVulnerabilityTypeHandler: Error creating vulnerability type: %v", err)
		// Check for unique constraint violation specifically if your DB layer returns a recognizable error
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Vulnerability type with this name already exists", http.StatusConflict)
		} else {
			http.Error(w, "Failed to create vulnerability type", http.StatusInternalServerError)
		}
		return
	}
	vt.ID = id
	// Re-fetch to get DB-generated timestamps
	createdVt, fetchErr := database.GetVulnerabilityTypeByID(id) // Assumes GetVulnerabilityTypeByID exists
	if fetchErr != nil {
		logger.Error("CreateVulnerabilityTypeHandler: Error fetching newly created vulnerability type %d: %v", id, fetchErr)
		// Fallback to returning the input with ID
		vt.CreatedAt = time.Now()
		vt.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(vt)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdVt)
}

// GetAllVulnerabilityTypesHandler handles GET requests to list all vulnerability types.
func GetAllVulnerabilityTypesHandler(w http.ResponseWriter, r *http.Request) {
	types, err := database.GetAllVulnerabilityTypes()
	if err != nil {
		logger.Error("GetAllVulnerabilityTypesHandler: Error fetching vulnerability types: %v", err)
		http.Error(w, "Failed to retrieve vulnerability types", http.StatusInternalServerError)
		return
	}
	if types == nil {
		types = []models.VulnerabilityType{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types)
}

// UpdateVulnerabilityTypeHandler handles PUT requests to update an existing vulnerability type.
func UpdateVulnerabilityTypeHandler(w http.ResponseWriter, r *http.Request) {
	vtIDStr := chi.URLParam(r, "vulnerability_type_id")
	vtID, err := strconv.ParseInt(vtIDStr, 10, 64)
	if err != nil {
		logger.Error("UpdateVulnerabilityTypeHandler: Invalid vulnerability_type_id format: %s", vtIDStr)
		http.Error(w, "Invalid vulnerability type ID format", http.StatusBadRequest)
		return
	}

	var req VulnerabilityTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("UpdateVulnerabilityTypeHandler: Error decoding request body for ID %d: %v", vtID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Vulnerability type name cannot be empty", http.StatusBadRequest)
		return
	}

	vtUpdate := models.VulnerabilityType{
		ID:          vtID,
		Name:        req.Name,
		Description: models.NullString(req.Description),
	}

	if err := database.UpdateVulnerabilityType(vtUpdate); err != nil {
		logger.Error("UpdateVulnerabilityTypeHandler: Error updating vulnerability type %d: %v", vtID, err)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Vulnerability type with this name already exists", http.StatusConflict)
		} else if strings.Contains(err.Error(), "not found") { // Assuming DB layer might return this
			http.Error(w, "Vulnerability type not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update vulnerability type", http.StatusInternalServerError)
		}
		return
	}
	// Re-fetch to get updated timestamps
	updatedVt, fetchErr := database.GetVulnerabilityTypeByID(vtID)
	if fetchErr != nil {
		logger.Error("UpdateVulnerabilityTypeHandler: Error fetching updated vulnerability type %d: %v", vtID, fetchErr)
		// Fallback
		vtUpdate.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(vtUpdate)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedVt)
}

// DeleteVulnerabilityTypeHandler handles DELETE requests to delete a vulnerability type.
func DeleteVulnerabilityTypeHandler(w http.ResponseWriter, r *http.Request) {
	vtIDStr := chi.URLParam(r, "vulnerability_type_id")
	vtID, err := strconv.ParseInt(vtIDStr, 10, 64)
	if err != nil {
		logger.Error("DeleteVulnerabilityTypeHandler: Invalid vulnerability_type_id format: %s", vtIDStr)
		http.Error(w, "Invalid vulnerability type ID format", http.StatusBadRequest)
		return
	}

	// Optional: Check if the type exists before attempting delete, to return 404 if not found.
	// _, fetchErr := database.GetVulnerabilityTypeByID(vtID)
	// if fetchErr != nil {
	// 	http.Error(w, "Vulnerability type not found", http.StatusNotFound)
	// 	return
	// }

	if err := database.DeleteVulnerabilityType(vtID); err != nil {
		logger.Error("DeleteVulnerabilityTypeHandler: Error deleting vulnerability type %d: %v", vtID, err)
		// Check for foreign key constraint violation if your DB layer returns a recognizable error
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			http.Error(w, "Cannot delete vulnerability type: it is currently associated with one or more findings.", http.StatusConflict)
		} else {
			http.Error(w, "Failed to delete vulnerability type", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetVulnerabilityTypeByID is a helper, not directly a handler, but used by handlers.
// It's good practice to have such helpers if they are complex or reused.
// For now, database.GetVulnerabilityTypeByID is simple enough.
// If needed, it would look like:
// func GetVulnerabilityTypeByID(id int64) (models.VulnerabilityType, error) {
//    return database.GetVulnerabilityTypeByID(id)
// }
