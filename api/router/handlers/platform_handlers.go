package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// createPlatform handles the creation of a new platform.
func createPlatform(w http.ResponseWriter, r *http.Request) {
	var p models.Platform
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		logger.Error("createPlatform: Error decoding request body: %v", err)
		// w.Header().Set("Content-Type", "application/json") // http.Error sets this, or json.NewEncoder does
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
		return
	}
	defer r.Body.Close()

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		logger.Error("createPlatform: Platform name is required and was empty")
		// w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Platform name is required"})
		return
	}

	createdPlatform, err := database.CreatePlatform(p.Name)
	if err != nil {
		// database.CreatePlatform should handle checking for existing name / unique constraint
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") ||
			strings.Contains(strings.ToLower(err.Error()), "already exists") { // More robust check
			logger.Error("createPlatform: Platform name '%s' conflicts: %v", p.Name, err)
			// w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform name '%s' already exists.", p.Name)})
		} else {
			logger.Error("createPlatform: Error inserting platform '%s': %v", p.Name, err)
			// w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while inserting platform"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdPlatform); err != nil {
		logger.Error("createPlatform: Error encoding response for platform '%s': %v", createdPlatform.Name, err)
	}
	logger.Info("Platform created: ID %d, Name '%s'", createdPlatform.ID, createdPlatform.Name)
}

// getPlatforms handles listing all platforms.
func getPlatforms(w http.ResponseWriter, r *http.Request) {
	platforms, err := database.GetAllPlatforms()
	if err != nil {
		logger.Error("getPlatforms: Error querying platforms: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(platforms); err != nil {
		logger.Error("getPlatforms: Error encoding response: %v", err)
	}
	logger.Info("Fetched %d platforms", len(platforms))
}

// GetPlatformByIDChiHandler is the chi-compatible handler for getting a platform by ID.
func GetPlatformByIDChiHandler(w http.ResponseWriter, r *http.Request) {
	platformIDStr := chi.URLParam(r, "platformID")
	platformID, err := strconv.ParseInt(platformIDStr, 10, 64)
	if err != nil {
		logger.Error("GetPlatformByIDChiHandler: Invalid platform ID format '%s': %v", platformIDStr, err)
		http.Error(w, "Invalid platform ID", http.StatusBadRequest)
		return
	}
	getPlatformByID(w, r, platformID)
}

// getPlatformByID handles fetching a single platform by its ID.
func getPlatformByID(w http.ResponseWriter, r *http.Request, platformID int64) {
	p, err := database.GetPlatformByID(platformID)
	if err != nil {
		// Check if the error message indicates "not found" as our db function now returns a specific error for that
		if strings.Contains(err.Error(), "not found") || errors.Is(err, sql.ErrNoRows) {
			logger.Error("getPlatformByID: Platform with ID %d not found", platformID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform with ID %d not found", platformID)})
		} else {
			logger.Error("getPlatformByID: Error querying platform ID %d: %v", platformID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(p); err != nil {
		logger.Error("getPlatformByID: Error encoding response for platform ID %d: %v", platformID, err)
	}
	logger.Info("Successfully retrieved platform ID %d", platformID)
}

// UpdatePlatformChiHandler is the chi-compatible handler for updating a platform.
func UpdatePlatformChiHandler(w http.ResponseWriter, r *http.Request) {
	platformIDStr := chi.URLParam(r, "platformID")
	platformID, err := strconv.ParseInt(platformIDStr, 10, 64)
	if err != nil {
		logger.Error("UpdatePlatformChiHandler: Invalid platform ID format '%s': %v", platformIDStr, err)
		http.Error(w, "Invalid platform ID", http.StatusBadRequest)
		return
	}
	updatePlatform(w, r, platformID)
}

// updatePlatform handles updating the name of an existing platform.
func updatePlatform(w http.ResponseWriter, r *http.Request, platformID int64) {
	var p models.Platform
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		logger.Error("updatePlatform: Error decoding request body for ID %d: %v", platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
		return
	}
	defer r.Body.Close()

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		logger.Error("updatePlatform: Platform name is required for update (ID: %d)", platformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Platform name is required"})
		return
	}

	updatedPlatform, err := database.UpdatePlatform(platformID, p.Name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("updatePlatform: Platform with ID %d not found for update", platformID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform with ID %d not found", platformID)})
		} else if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") ||
			strings.Contains(strings.ToLower(err.Error()), "already exists") {
			logger.Error("updatePlatform: Update for ID %d failed due to UNIQUE constraint (name '%s'): %v", platformID, p.Name, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Update failed because the name '%s' already exists.", p.Name)})
		} else {
			logger.Error("updatePlatform: Error executing update for ID %d: %v", platformID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while updating platform"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedPlatform); err != nil {
		logger.Error("updatePlatform: Error encoding response for platform ID %d: %v", platformID, err)
	}
	logger.Info("Platform updated: ID %d, New Name '%s'", platformID, p.Name)
}

// DeletePlatformChiHandler is the chi-compatible handler for deleting a platform.
func DeletePlatformChiHandler(w http.ResponseWriter, r *http.Request) {
	platformIDStr := chi.URLParam(r, "platformID")
	platformID, err := strconv.ParseInt(platformIDStr, 10, 64)
	if err != nil {
		logger.Error("DeletePlatformChiHandler: Invalid platform ID format '%s': %v", platformIDStr, err)
		http.Error(w, "Invalid platform ID", http.StatusBadRequest)
		return
	}
	deletePlatform(w, r, platformID)
}

// deletePlatform handles deleting a specific platform by its ID.
func deletePlatform(w http.ResponseWriter, r *http.Request, platformID int64) {
	logger.Info("Attempting to delete platform with ID %d", platformID)
	err := database.DeletePlatform(platformID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("deletePlatform: Platform with ID %d not found for deletion", platformID)
			http.Error(w, fmt.Sprintf("Platform with ID %d not found", platformID), http.StatusNotFound)
		} else {
			logger.Error("deletePlatform: Error deleting platform ID %d: %v", platformID, err)
			http.Error(w, "Internal server error during delete", http.StatusInternalServerError)
		}
		return
	}

	logger.Info("Platform deleted successfully: ID %d", platformID)
	w.WriteHeader(http.StatusNoContent)
}
