package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http" // Keep for isValidURL if it uses regex, or remove if not. Current isValidURL doesn't.
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// validScopeItemTypes is used for validating input before it's passed to the database layer.
var validScopeItemTypes = map[string]bool{"domain": true, "subdomain": true, "ip_address": true, "cidr": true, "url_path": true}

// isValidURL checks if the string is a somewhat valid URL.
// This can be enhanced for more robust validation if needed.
func isValidURL(toTest string) bool {
	// Basic check: starts with http/https or contains a dot (common for domains) and no spaces.
	// This is a simplified check. For strict URL validation, consider a library or more complex regex.
	return strings.HasPrefix(toTest, "http://") || strings.HasPrefix(toTest, "https://") ||
		(strings.Contains(toTest, ".") && !strings.ContainsAny(toTest, " \t\n"))
}

// createTarget handles the creation of a new target.
// It now calls database.CreateTargetWithScopeRules for the core logic.
func createTarget(w http.ResponseWriter, r *http.Request) {
	var req models.TargetCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("createTarget: Error decoding request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid request body: " + err.Error()})
		return
	}
	defer r.Body.Close()

	// Basic input validation in the handler
	if req.PlatformID == 0 {
		logger.Error("createTarget: PlatformID is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "PlatformID is required"})
		return
	}
	req.Codename = strings.TrimSpace(req.Codename)
	if req.Codename == "" {
		logger.Error("createTarget: Codename is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Codename is required"})
		return
	}
	req.Link = strings.TrimSpace(req.Link)
	if req.Link == "" {
		logger.Error("createTarget: Link is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Link is required"})
		return
	}
	if !isValidURL(req.Link) { // Ensure isValidURL is defined in this file or package
		logger.Error("createTarget: Invalid Link format: %s", req.Link)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Link must be a valid URL"})
		return
	}
	// Validate scope item types before passing to DB layer
	allScopeItems := append(req.InScopeItems, req.OutOfScopeItems...)
	for _, item := range allScopeItems {
		itemType := strings.ToLower(strings.TrimSpace(item.ItemType))
		if itemType != "" && !validScopeItemTypes[itemType] { // Only validate if provided, DB layer will determine if empty
			logger.Error("createTarget: Invalid item_type '%s' for pattern '%s'", item.ItemType, item.Pattern)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Invalid item_type '%s' provided for scope rule.", item.ItemType)})
			return
		}
	}

	createdTarget, err := database.CreateTargetWithScopeRules(req)
	if err != nil {
		// database.CreateTargetWithScopeRules handles specific error types like "platform not found" or "codename exists"
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "conflicts") {
			logger.Error("createTarget: Conflict creating target: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: err.Error()})
		} else if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "invalid item_type") {
			logger.Error("createTarget: Bad request creating target: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: err.Error()})
		} else {
			logger.Error("createTarget: Internal server error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdTarget); err != nil {
		logger.Error("createTarget: Error encoding response for target '%s': %v", createdTarget.Codename, err)
	}
	logger.Info("Target created: ID %d, Codename '%s', PlatformID %d, Slug '%s', with %d scope rules",
		createdTarget.ID, createdTarget.Codename, createdTarget.PlatformID, createdTarget.Slug, len(createdTarget.ScopeRules))
}

// getTargets handles listing targets.
func getTargets(w http.ResponseWriter, r *http.Request) {
	platformIDStr := r.URL.Query().Get("platform_id")
	var platformIDFilter *int64

	if platformIDStr != "" {
		pid, err := strconv.ParseInt(platformIDStr, 10, 64)
		if err != nil {
			logger.Error("getTargets: Invalid platform_id parameter '%s': %v", platformIDStr, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid platform_id parameter"})
			return
		}
		platformIDFilter = &pid
	}

	targets, err := database.GetTargets(platformIDFilter)
	if err != nil {
		logger.Error("getTargets: Error querying targets: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(targets); err != nil {
		logger.Error("getTargets: Error encoding response: %v", err)
	}
	if platformIDFilter != nil {
		logger.Info("Fetched %d targets for platform_id %d", len(targets), *platformIDFilter)
	} else {
		logger.Info("Fetched %d targets", len(targets))
	}
}

// GetTargetByIDChiHandler is the chi-compatible handler for getting a target by ID.
// It expects idOrSlug to be a numeric ID.
func GetTargetByIDChiHandler(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	targetID, err := strconv.ParseInt(idOrSlug, 10, 64)
	if err != nil {
		logger.Error("GetTargetByIDChiHandler: Invalid target ID format '%s': %v", idOrSlug, err)
		http.Error(w, "Invalid target ID (must be numeric for this endpoint)", http.StatusBadRequest)
		return
	}
	GetTargetByID(w, r, targetID)
}

// UpdateTargetDetailsHandler handles PUT requests to update a target's details.
func UpdateTargetDetailsHandler(w http.ResponseWriter, r *http.Request, targetID int64) {
	var req models.TargetUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("UpdateTargetDetailsHandler: Error decoding request body for target ID %d: %v", targetID, err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	req.Link = strings.TrimSpace(req.Link)
	if req.Link == "" {
		logger.Error("UpdateTargetDetailsHandler: Link is required for target ID %d.", targetID)
		http.Error(w, "Link is required.", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(req.Link, "#") && !isValidURL(req.Link) { // Ensure isValidURL is defined
		logger.Error("UpdateTargetDetailsHandler: Invalid Link format '%s' for target ID %d.", req.Link, targetID)
		http.Error(w, "Link must be a valid URL (e.g., http://example.com) or a placeholder starting with '#'.", http.StatusBadRequest)
		return
	}

	err := database.UpdateTargetDetails(targetID, req.Link, req.Notes)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("UpdateTargetDetailsHandler: Target with ID %d not found for update.", targetID)
			http.Error(w, fmt.Sprintf("Target with ID %d not found.", targetID), http.StatusNotFound)
		} else {
			logger.Error("UpdateTargetDetailsHandler: Error executing update for target ID %d: %v", targetID, err)
			http.Error(w, "Internal server error during update", http.StatusInternalServerError)
		}
		return
	}

	logger.Info("Successfully updated details for target ID %d. Link: '%s'", targetID, req.Link)
	GetTargetByID(w, r, targetID) // Respond with the updated target details
}

// UpdateTargetDetailsChiHandler is the chi-compatible handler for updating target details.
func UpdateTargetDetailsChiHandler(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	targetID, err := strconv.ParseInt(idOrSlug, 10, 64)
	if err != nil {
		logger.Error("UpdateTargetDetailsChiHandler: Invalid target ID format '%s': %v", idOrSlug, err)
		http.Error(w, "Invalid target ID (must be numeric for update)", http.StatusBadRequest)
		return
	}
	UpdateTargetDetailsHandler(w, r, targetID)
}

// DeleteTarget handles deleting a target by its ID or slug.
func DeleteTarget(w http.ResponseWriter, r *http.Request, identifier string) {
	deleted, err := database.DeleteTargetByIDOrSlug(identifier)
	if err != nil {
		logger.Error("DeleteTarget: Error deleting target '%s': %v", identifier, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		return
	}

	if !deleted {
		logger.Error("DeleteTarget: Target '%s' not found for deletion", identifier)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target '%s' not found", identifier)})
		return
	}

	logger.Info("Target deleted successfully: %s", identifier)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteTargetChiHandler is the chi-compatible handler for deleting a target.
func DeleteTargetChiHandler(w http.ResponseWriter, r *http.Request) {
	idOrSlug := chi.URLParam(r, "idOrSlug")
	DeleteTarget(w, r, idOrSlug)
}

// DeleteTargetByCodenameHandler handles deleting a target by its codename and platform_id.
func DeleteTargetByCodenameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		logger.Error("DeleteTargetByCodenameHandler: MethodNotAllowed: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Only DELETE method is allowed"})
		return
	}
	codename := r.URL.Query().Get("codename")
	platformIDStr := r.URL.Query().Get("platform_id")

	if codename == "" || platformIDStr == "" {
		logger.Error("DeleteTargetByCodenameHandler: 'codename' and 'platform_id' query parameters are required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "'codename' and 'platform_id' query parameters are required"})
		return
	}

	platformID, err := strconv.ParseInt(platformIDStr, 10, 64)
	if err != nil {
		logger.Error("DeleteTargetByCodenameHandler: Invalid 'platform_id': %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid 'platform_id' parameter"})
		return
	}

	deleted, err := database.DeleteTargetByCodenameAndPlatform(platformID, codename)
	if err != nil {
		logger.Error("DeleteTargetByCodenameHandler: Error deleting target by codename '%s', platform %d: %v", codename, platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		return
	}

	if !deleted {
		logger.Error("DeleteTargetByCodenameHandler: No target found with codename '%s' for platform ID %d", codename, platformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("No target found with codename '%s' for platform ID %d", codename, platformID)})
		return
	}

	logger.Info("Target deleted successfully by codename '%s', platform ID %d", codename, platformID)
	w.WriteHeader(http.StatusNoContent)
}

// GetTargetByID handles fetching a single target by its ID, including its scope rules.
func GetTargetByID(w http.ResponseWriter, r *http.Request, targetID int64) {
	t, err := database.GetTargetByID(targetID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("GetTargetByID: Target with ID %d not found", targetID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target with ID %d not found", targetID)})
		} else {
			logger.Error("GetTargetByID: Error querying target ID %d: %v", targetID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(t); err != nil {
		logger.Error("GetTargetByID: Error encoding response for target ID %d: %v", targetID, err)
	}
	logger.Info("Fetched target by ID: %d, Codename: '%s', with %d scope rules", t.ID, t.Codename, len(t.ScopeRules))
}

// PromoteSynackTargetHandler handles promoting a Synack target to the main targets table.
func PromoteSynackTargetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("PromoteSynackTargetHandler: MethodNotAllowed: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed"})
		return
	}

	var req models.PromoteSynackTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("PromoteSynackTargetHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.SynackTargetIDStr == "" || req.PlatformID == 0 {
		logger.Error("PromoteSynackTargetHandler: synack_target_id_str and platform_id are required.")
		http.Error(w, "synack_target_id_str and platform_id are required.", http.StatusBadRequest)
		return
	}

	// 1. Fetch Synack target details
	// This part should ideally call a function in synack_db.go, e.g., database.GetSynackTargetBySynackIDStr()
	// For now, keeping the direct DB call here as it was, but it's a candidate for refactoring.
	var st models.SynackTarget
	var codename, name sql.NullString // Use sql.NullString for nullable fields
	err := database.DB.QueryRow(`SELECT id, synack_target_id_str, codename, name FROM synack_targets WHERE synack_target_id_str = ?`, req.SynackTargetIDStr).Scan(
		&st.DBID, &st.SynackTargetIDStr, &codename, &name,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("PromoteSynackTargetHandler: Synack target with ID_str '%s' not found.", req.SynackTargetIDStr)
			http.Error(w, fmt.Sprintf("Synack target with ID '%s' not found.", req.SynackTargetIDStr), http.StatusNotFound)
		} else {
			logger.Error("PromoteSynackTargetHandler: Error fetching Synack target '%s': %v", req.SynackTargetIDStr, err)
			http.Error(w, "Internal server error fetching Synack target.", http.StatusInternalServerError)
		}
		return
	}
	if codename.Valid {
		st.Codename = codename.String
	}
	if name.Valid {
		st.Name = name.String
	}

	mainTargetCodename := req.CodenameOverride
	if mainTargetCodename == "" {
		mainTargetCodename = st.Codename
		if mainTargetCodename == "" {
			mainTargetCodename = st.Name
		}
	}
	mainTargetCodename = strings.TrimSpace(mainTargetCodename)
	if mainTargetCodename == "" {
		logger.Error("PromoteSynackTargetHandler: Could not determine a codename for the new target.")
		http.Error(w, "Codename for the new target is required (either override or from Synack data).", http.StatusBadRequest)
		return
	}

	mainTargetLink := req.LinkOverride
	if mainTargetLink == "" {
		logger.Error("PromoteSynackTargetHandler: Link for the new target is required via link_override.")
		http.Error(w, "link_override is required when promoting a Synack target.", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(mainTargetLink, "#") && !isValidURL(mainTargetLink) { // Ensure isValidURL is defined
		logger.Error("PromoteSynackTargetHandler: Invalid link_override format: %s. Must be a valid URL or a placeholder starting with '#'.", mainTargetLink)
		http.Error(w, "link_override must be a valid URL or a placeholder starting with '#'.", http.StatusBadRequest)
		return
	}

	targetCreateData := models.TargetCreateRequest{
		PlatformID:      req.PlatformID,
		Codename:        mainTargetCodename,
		Link:            mainTargetLink,
		Notes:           req.NotesOverride,
		InScopeItems:    req.InScopeItemsOverride,
		OutOfScopeItems: req.OutOfScopeItemsOverride,
	}

	createdTarget, err := database.CreateTargetWithScopeRules(targetCreateData)
	if err != nil {
		// database.CreateTargetWithScopeRules handles specific error types
		if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "conflicts") {
			logger.Error("PromoteSynackTargetHandler: Conflict creating main target: %v", err)
			http.Error(w, err.Error(), http.StatusConflict)
		} else if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "invalid item_type") {
			logger.Error("PromoteSynackTargetHandler: Bad request creating main target: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			logger.Error("PromoteSynackTargetHandler: Internal server error creating main target: %v", err)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
		}
		return
	}

	// Update Synack target status - this should ideally be a function in synack_db.go
	// For now, direct DB call for simplicity of this step.
	_, errUpdateSynack := database.DB.Exec("UPDATE synack_targets SET status = ?, last_seen_timestamp = ? WHERE synack_target_id_str = ?",
		"promoted", time.Now().UTC().Format(time.RFC3339), req.SynackTargetIDStr)
	if errUpdateSynack != nil {
		// Log this error, but the main target creation was successful, so proceed with 201.
		logger.Error("PromoteSynackTargetHandler: Error updating status for Synack target '%s' (main target created successfully): %v", req.SynackTargetIDStr, errUpdateSynack)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(createdTarget); err != nil {
		logger.Error("PromoteSynackTargetHandler: Error encoding response: %v", err)
	}
	logger.Info("Successfully promoted Synack target '%s' to main target ID %d (Codename: '%s')", req.SynackTargetIDStr, createdTarget.ID, createdTarget.Codename)
}

// GetChecklistItemsForTargetChiHandler is the chi-compatible handler for getting checklist items for a target.
func GetChecklistItemsForTargetChiHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "idOrSlug") // The route group uses {idOrSlug}
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("GetChecklistItemsForTargetChiHandler: Invalid target ID format '%s': %v", targetIDStr, err)
		http.Error(w, "Invalid target ID for checklist items (must be numeric)", http.StatusBadRequest)
		return
	}
	GetChecklistItemsHandler(w, r, targetID) // This function is in checklist_handlers.go
}
