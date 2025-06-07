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
)

// PlatformsCollectionHandler handles requests for the /platforms collection path.
// It dispatches to getPlatforms for GET and createPlatform for POST.
func PlatformsCollectionHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("PlatformsCollectionHandler: Method=%s, Received Path (relative to /api)='%s'", r.Method, r.URL.Path)

	if r.URL.Path == "/platforms" {
		switch r.Method {
		case http.MethodGet:
			logger.Debug("PlatformsCollectionHandler: Routing to getPlatforms for path '%s'", r.URL.Path)
			getPlatforms(w, r)
		case http.MethodPost:
			logger.Debug("PlatformsCollectionHandler: Routing to createPlatform for path '%s'", r.URL.Path)
			createPlatform(w, r)
		default:
			logger.Error("PlatformsCollectionHandler: MethodNotAllowed: %s for /platforms", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed for platform collection"})
		}
	} else {
		logger.Error("PlatformsCollectionHandler: Path mismatch or not an exact match for collection. Path received: '%s'. This should have been caught by a more specific item handler or the catch-all.", r.URL.Path)
		http.NotFound(w, r)
	}
}

// PlatformItemHandler handles requests for a specific platform item, e.g., /platforms/{id}
func PlatformItemHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("PlatformItemHandler: Method=%s, Received Path (relative to /api)='%s'", r.Method, r.URL.Path)

	idStr := strings.TrimPrefix(r.URL.Path, "/platforms/")
	idStr = strings.Trim(idStr, "/")

	if idStr == "" {
		logger.Error("PlatformItemHandler: Path was '/platforms/', which is ambiguous. Expected /platforms/{id}. This might indicate a routing issue or incorrect client request.")
		http.NotFound(w, r)
		return
	}

	platformID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.Error("PlatformItemHandler: Invalid platform ID format '%s' in path: %v", idStr, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid platform ID format. Must be numeric."})
		return
	}

	logger.Info("PlatformItemHandler: Parsed PlatformID %d", platformID)

	switch r.Method {
	case http.MethodGet:
		getPlatformByID(w, r, platformID)
	case http.MethodPut:
		updatePlatform(w, r, platformID)
	case http.MethodDelete:
		deletePlatform(w, r, platformID)
	default:
		logger.Error("PlatformItemHandler: MethodNotAllowed: %s for /platforms/%d", r.Method, platformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed for this platform resource"})
	}
}

// createPlatform handles the creation of a new platform.
func createPlatform(w http.ResponseWriter, r *http.Request) {
	var p models.Platform
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		logger.Error("createPlatform: Error decoding request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
		return
	}
	defer r.Body.Close()

	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		logger.Error("createPlatform: Platform name is required and was empty")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Platform name is required"})
		return
	}

	var existingID int64
	err := database.DB.QueryRow("SELECT id FROM platforms WHERE LOWER(name) = LOWER(?)", p.Name).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("createPlatform: Error checking for existing platform '%s': %v", p.Name, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while checking platform"})
		return
	}
	if err == nil {
		logger.Error("createPlatform: Platform '%s' already exists with ID %d", p.Name, existingID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform '%s' already exists", p.Name)})
		return
	}

	stmt, err := database.DB.Prepare("INSERT INTO platforms(name) VALUES(?)")
	if err != nil {
		logger.Error("createPlatform: Error preparing statement for platform '%s': %v", p.Name, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	res, err := stmt.Exec(p.Name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			logger.Error("createPlatform: Platform name '%s' conflicts (UNIQUE constraint).", p.Name)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform name '%s' already exists.", p.Name)})
		} else {
			logger.Error("createPlatform: Error inserting platform '%s': %v", p.Name, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while inserting platform"})
		}
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		logger.Error("createPlatform: Error getting last insert ID for platform '%s': %v", p.Name, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error after inserting platform"})
		return
	}
	p.ID = id

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		logger.Error("createPlatform: Error encoding response for platform '%s': %v", p.Name, err)
	}
	logger.Info("Platform created: ID %d, Name '%s'", p.ID, p.Name)
}

// getPlatforms handles listing all platforms.
func getPlatforms(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name FROM platforms ORDER BY name ASC")
	if err != nil {
		logger.Error("getPlatforms: Error querying platforms: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer rows.Close()

	platforms := []models.Platform{}
	for rows.Next() {
		var p models.Platform
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			logger.Error("getPlatforms: Error scanning platform row: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
			return
		}
		platforms = append(platforms, p)
	}
	if err = rows.Err(); err != nil {
		logger.Error("getPlatforms: Error iterating platform rows: %v", err)
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

// getPlatformByID handles fetching a single platform by its ID.
func getPlatformByID(w http.ResponseWriter, r *http.Request, platformID int64) {
	var p models.Platform
	query := `SELECT id, name FROM platforms WHERE id = ?`
	err := database.DB.QueryRow(query, platformID).Scan(&p.ID, &p.Name)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	var existingID int64
	err := database.DB.QueryRow("SELECT id FROM platforms WHERE LOWER(name) = LOWER(?) AND id != ?", p.Name, platformID).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("updatePlatform: Error checking for conflicting new name '%s' (for ID %d): %v", p.Name, platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error checking name conflict"})
		return
	}
	if err == nil {
		logger.Error("updatePlatform: New name '%s' conflicts with existing platform ID %d (while updating ID %d)", p.Name, existingID, platformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Another platform already exists with the name '%s'", p.Name)})
		return
	}

	stmt, err := database.DB.Prepare("UPDATE platforms SET name = ? WHERE id = ?")
	if err != nil {
		logger.Error("updatePlatform: Error preparing update statement for ID %d: %v", platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(p.Name, platformID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
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

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("updatePlatform: Platform with ID %d not found for update", platformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform with ID %d not found", platformID)})
		return
	}

	updatedPlatform := models.Platform{ID: platformID, Name: p.Name}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedPlatform); err != nil {
		logger.Error("updatePlatform: Error encoding response for platform ID %d: %v", platformID, err)
	}
	logger.Info("Platform updated: ID %d, New Name '%s'", platformID, p.Name)
}

// deletePlatform handles deleting a specific platform by its ID.
func deletePlatform(w http.ResponseWriter, r *http.Request, platformID int64) {
	logger.Info("Attempting to delete platform with ID %d", platformID)

	stmt, err := database.DB.Prepare("DELETE FROM platforms WHERE id = ?")
	if err != nil {
		logger.Error("deletePlatform: Error preparing delete statement for ID %d: %v", platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(platformID)
	if err != nil {
		logger.Error("deletePlatform: Error executing delete for ID %d: %v", platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("deletePlatform: Error getting rows affected for ID %d: %v", platformID, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if rowsAffected == 0 {
		logger.Error("deletePlatform: Platform with ID %d not found for deletion", platformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform with ID %d not found", platformID)})
		return
	}

	logger.Info("Platform deleted successfully: ID %d", platformID)
	w.WriteHeader(http.StatusNoContent)
}
