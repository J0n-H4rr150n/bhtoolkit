package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/google/uuid"
)

var validScopeItemTypes = map[string]bool{"domain": true, "subdomain": true, "ip_address": true, "cidr": true, "url_path": true}

// slugify creates a URL-friendly slug from a string.
func slugify(s string) string {
	slug := strings.ToLower(s)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return uuid.New().String()[:8]
	}
	return slug
}

// isValidURL checks if the string is a somewhat valid URL.
func isValidURL(toTest string) bool {
	return strings.HasPrefix(toTest, "http://") || strings.HasPrefix(toTest, "https://") || (strings.Contains(toTest, ".") && !strings.ContainsAny(toTest, " \t\n"))
}

// determineItemType infers item_type from pattern for scope rules.
func determineItemType(pattern string) string {
	if strings.HasPrefix(pattern, "/") {
		return "url_path"
	}
	if regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(/\d{1,2})?$`).MatchString(pattern) {
		if strings.Contains(pattern, "/") {
			return "cidr"
		}
		return "ip_address"
	}
	if strings.Contains(pattern, ".") && !strings.ContainsAny(pattern, " /") {
		if strings.HasPrefix(pattern, "*.") {
			return "subdomain"
		}
		return "domain"
	}
	return "domain"
}

// TargetsHandler handles requests for the /api/targets path.
func TargetsHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("TargetsHandler received request: Method %s, Relative Path %s", r.Method, r.URL.Path)
	switch r.Method {
	case http.MethodPost:
		createTarget(w, r)
	case http.MethodGet:
		getTargets(w, r)
	default:
		logger.Error("TargetsHandler: MethodNotAllowed: %s for /targets", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Method not allowed"})
	}
}

// createTarget handles the creation of a new target.
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
	if !isValidURL(req.Link) {
		logger.Error("createTarget: Invalid Link format: %s", req.Link)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Link must be a valid URL"})
		return
	}

	var platformExists bool
	err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM platforms WHERE id = ?)", req.PlatformID).Scan(&platformExists)
	if err != nil {
		logger.Error("createTarget: Error checking platform existence for PlatformID %d: %v", req.PlatformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while verifying platform"})
		return
	}
	if !platformExists {
		logger.Error("createTarget: PlatformID %d does not exist", req.PlatformID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Platform with ID %d does not exist", req.PlatformID)})
		return
	}

	var existingTargetID int64
	err = database.DB.QueryRow("SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)", req.PlatformID, req.Codename).Scan(&existingTargetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("createTarget: Error checking for existing target codename '%s' under platform %d: %v", req.Codename, req.PlatformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while checking for duplicate target"})
		return
	}
	if err == nil {
		logger.Error("createTarget: Target with codename '%s' already exists for platform ID %d (existing target ID: %d)", req.Codename, req.PlatformID, existingTargetID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target codename '%s' already exists for this platform.", req.Codename)})
		return
	}

	generatedSlug := slugify(req.Codename)
	var existingSlugID int64
	err = database.DB.QueryRow("SELECT id FROM targets WHERE slug = ?", generatedSlug).Scan(&existingSlugID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("createTarget: Error checking for existing slug '%s': %v", generatedSlug, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while checking slug"})
		return
	}
	if err == nil {
		generatedSlug = fmt.Sprintf("%s-%s", generatedSlug, uuid.New().String()[:4])
	}

	tx, err := database.DB.Begin()
	if err != nil {
		logger.Error("createTarget: Error beginning database transaction: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	stmt, err := tx.Prepare(`INSERT INTO targets (platform_id, slug, codename, link, notes) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		logger.Error("createTarget: Error preparing target insert statement for '%s': %v", req.Codename, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	res, err := stmt.Exec(req.PlatformID, generatedSlug, req.Codename, req.Link, req.Notes)
	if err != nil {
		tx.Rollback()
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			logger.Error("createTarget: Target '%s' under platform %d conflicts (UNIQUE constraint).", req.Codename, req.PlatformID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target codename '%s' already exists for this platform.", req.Codename)})
		} else if strings.Contains(err.Error(), "UNIQUE constraint failed") && strings.Contains(err.Error(), "targets.slug") {
			logger.Error("createTarget: Slug '%s' for target '%s' conflicts (UNIQUE constraint).", generatedSlug, req.Codename)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Generated slug '%s' conflicts.", generatedSlug)})
		} else {
			logger.Error("createTarget: Error inserting target '%s': %v", req.Codename, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while inserting target"})
		}
		stmt.Close()
		return
	}
	stmt.Close()

	targetID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		logger.Error("createTarget: Error getting last insert ID for target '%s': %v", req.Codename, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error after inserting target"})
		return
	}

	scopeStmt, err := tx.Prepare(`INSERT INTO scope_rules 
        (target_id, item_type, pattern, is_in_scope, is_wildcard, description) 
        VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		logger.Error("createTarget: Error preparing scope_rules insert statement for target ID %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer scopeStmt.Close()

	createdScopeRules := []models.ScopeRule{}
	allScopeItems := append(req.InScopeItems, req.OutOfScopeItems...)
	isInScopeFlag := true

	for i, item := range allScopeItems {
		if i >= len(req.InScopeItems) { // Switch to out-of-scope items
			isInScopeFlag = false
		}
		itemType := strings.ToLower(strings.TrimSpace(item.ItemType))
		if itemType == "" {
			itemType = determineItemType(item.Pattern)
		} else if !validScopeItemTypes[itemType] {
			tx.Rollback()
			logger.Error("createTarget: Invalid item_type '%s' for pattern '%s' for target ID %d", item.ItemType, item.Pattern, targetID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Invalid item_type '%s' provided for scope rule.", item.ItemType)})
			return
		}
		isWildcard := strings.Contains(item.Pattern, "*")
		_, err := scopeStmt.Exec(targetID, itemType, item.Pattern, isInScopeFlag, isWildcard, item.Description)
		if err != nil {
			tx.Rollback()
			logger.Error("createTarget: Error inserting scope rule '%s' (in_scope: %t) for target ID %d: %v", item.Pattern, isInScopeFlag, targetID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while inserting scope rules"})
			return
		}
		createdScopeRules = append(createdScopeRules, models.ScopeRule{TargetID: targetID, ItemType: itemType, Pattern: item.Pattern, IsInScope: isInScopeFlag, IsWildcard: isWildcard, Description: item.Description})
	}

	if err := tx.Commit(); err != nil {
		logger.Error("createTarget: Error committing transaction for target '%s': %v", req.Codename, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	responseTarget := models.Target{
		ID: targetID, PlatformID: req.PlatformID, Slug: generatedSlug, Codename: req.Codename,
		Link: req.Link, Notes: req.Notes, ScopeRules: createdScopeRules,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(responseTarget); err != nil {
		logger.Error("createTarget: Error encoding response for target '%s': %v", req.Codename, err)
	}
	logger.Info("Target created: ID %d, Codename '%s', PlatformID %d, Slug '%s', with %d scope rules",
		targetID, req.Codename, req.PlatformID, generatedSlug, len(createdScopeRules))
}

// getTargets handles listing targets.
func getTargets(w http.ResponseWriter, r *http.Request) {
	platformIDStr := r.URL.Query().Get("platform_id")
	var platformID int64
	var err error

	query := "SELECT id, platform_id, slug, codename, link, notes FROM targets"
	args := []interface{}{}

	if platformIDStr != "" {
		platformID, err = strconv.ParseInt(platformIDStr, 10, 64)
		if err != nil {
			logger.Error("getTargets: Invalid platform_id parameter '%s': %v", platformIDStr, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Invalid platform_id parameter"})
			return
		}
		query += " WHERE platform_id = ?"
		args = append(args, platformID)
	}
	query += " ORDER BY codename ASC"

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		logger.Error("getTargets: Error querying targets: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer rows.Close()

	targets := []models.Target{}
	for rows.Next() {
		var t models.Target
		var slug sql.NullString
		var notes sql.NullString
		if err := rows.Scan(&t.ID, &t.PlatformID, &slug, &t.Codename, &t.Link, &notes); err != nil {
			logger.Error("getTargets: Error scanning target row: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
			return
		}
		if slug.Valid {
			t.Slug = slug.String
		}
		if notes.Valid {
			t.Notes = notes.String
		}
		targets = append(targets, t)
	}
	if err = rows.Err(); err != nil {
		logger.Error("getTargets: Error iterating target rows: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(targets); err != nil {
		logger.Error("getTargets: Error encoding response: %v", err)
	}
	if platformIDStr != "" {
		logger.Info("Fetched %d targets for platform_id %d", len(targets), platformID)
	} else {
		logger.Info("Fetched %d targets", len(targets))
	}
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
	if !strings.HasPrefix(req.Link, "#") && !isValidURL(req.Link) {
		logger.Error("UpdateTargetDetailsHandler: Invalid Link format '%s' for target ID %d.", req.Link, targetID)
		http.Error(w, "Link must be a valid URL (e.g., http://example.com) or a placeholder starting with '#'.", http.StatusBadRequest)
		return
	}

	stmt, err := database.DB.Prepare("UPDATE targets SET link = ?, notes = ? WHERE id = ?")
	if err != nil {
		logger.Error("UpdateTargetDetailsHandler: Error preparing update statement for target ID %d: %v", targetID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(req.Link, req.Notes, targetID)
	if err != nil {
		logger.Error("UpdateTargetDetailsHandler: Error executing update for target ID %d: %v", targetID, err)
		http.Error(w, "Internal server error during update", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		logger.Error("UpdateTargetDetailsHandler: Target with ID %d not found for update.", targetID)
		http.Error(w, fmt.Sprintf("Target with ID %d not found.", targetID), http.StatusNotFound)
		return
	}

	logger.Info("Successfully updated details for target ID %d. Link: '%s'", targetID, req.Link)
	GetTargetByID(w, r, targetID) // Respond with the updated target details
}

// DeleteTarget handles deleting a target by its ID or slug.
func DeleteTarget(w http.ResponseWriter, r *http.Request, identifier string) {
	var result sql.Result
	var err error
	var deletedBy string
	var query string
	var args []interface{}

	targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
	if parseErr == nil {
		deletedBy = fmt.Sprintf("ID %d", targetID)
		query = "DELETE FROM targets WHERE id = ?"
		args = append(args, targetID)
	} else {
		deletedBy = fmt.Sprintf("slug '%s'", identifier)
		query = "DELETE FROM targets WHERE slug = ?"
		args = append(args, identifier)
	}

	logger.Info("Attempting to delete target by %s", deletedBy)

	stmt, errPrep := database.DB.Prepare(query)
	if errPrep != nil {
		logger.Error("DeleteTarget: Error preparing delete statement for target %s: %v", deletedBy, errPrep)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	result, err = stmt.Exec(args...)
	if err != nil {
		logger.Error("DeleteTarget: Error executing delete statement for target %s: %v", deletedBy, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Error("DeleteTarget: Error getting rows affected for target %s: %v", deletedBy, err)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if rowsAffected == 0 {
		logger.Error("DeleteTarget: Target %s not found for deletion", deletedBy)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: fmt.Sprintf("Target %s not found", deletedBy)})
		return
	}

	logger.Info("Target deleted successfully: %s", deletedBy)
	w.WriteHeader(http.StatusNoContent)
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

	logger.Info("Attempting to delete target by codename '%s' for platform ID %d", codename, platformID)

	stmt, err := database.DB.Prepare("DELETE FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)")
	if err != nil {
		logger.Error("DeleteTargetByCodenameHandler: Error preparing statement: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error"})
		return
	}
	defer stmt.Close()

	result, err := stmt.Exec(platformID, codename)
	if err != nil {
		logger.Error("DeleteTargetByCodenameHandler: Error executing delete for codename '%s', platform %d: %v", codename, platformID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error during delete"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
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
	var t models.Target
	err := database.DB.QueryRow(`SELECT id, platform_id, slug, codename, link, notes 
                                 FROM targets WHERE id = ?`, targetID).Scan(
		&t.ID, &t.PlatformID, &t.Slug, &t.Codename, &t.Link, &t.Notes,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	scopeRows, err := database.DB.Query(`SELECT id, item_type, pattern, is_in_scope, is_wildcard, description 
                                        FROM scope_rules WHERE target_id = ? ORDER BY id ASC`, targetID)
	if err != nil {
		logger.Error("GetTargetByID: Error querying scope rules for target ID %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while fetching scope rules"})
		return
	}
	defer scopeRows.Close()

	t.ScopeRules = []models.ScopeRule{}
	for scopeRows.Next() {
		var sr models.ScopeRule
		sr.TargetID = targetID
		if err := scopeRows.Scan(&sr.ID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description); err != nil {
			logger.Error("GetTargetByID: Error scanning scope rule row for target ID %d: %v", targetID, err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while scanning scope rules"})
			return
		}
		t.ScopeRules = append(t.ScopeRules, sr)
	}
	if err = scopeRows.Err(); err != nil {
		logger.Error("GetTargetByID: Error iterating scope rule rows for target ID %d: %v", targetID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(models.ErrorResponse{Message: "Internal server error while iterating scope rules"})
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

	var st models.SynackTarget
	var codename, name sql.NullString

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

	var platformExists bool
	err = database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM platforms WHERE id = ?)", req.PlatformID).Scan(&platformExists)
	if err != nil {
		logger.Error("PromoteSynackTargetHandler: Error checking platform existence for PlatformID %d: %v", req.PlatformID, err)
		http.Error(w, "Internal server error verifying platform.", http.StatusInternalServerError)
		return
	}
	if !platformExists {
		logger.Error("PromoteSynackTargetHandler: PlatformID %d does not exist for main target.", req.PlatformID)
		http.Error(w, fmt.Sprintf("Platform with ID %d does not exist.", req.PlatformID), http.StatusBadRequest)
		return
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
	if !strings.HasPrefix(mainTargetLink, "#") && !isValidURL(mainTargetLink) {
		logger.Error("PromoteSynackTargetHandler: Invalid link_override format: %s. Must be a valid URL or a placeholder starting with '#'.", mainTargetLink)
		http.Error(w, "link_override must be a valid URL or a placeholder starting with '#'.", http.StatusBadRequest)
		return
	}

	mainTargetNotes := req.NotesOverride

	var existingMainTargetID int64
	err = database.DB.QueryRow("SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)", req.PlatformID, mainTargetCodename).Scan(&existingMainTargetID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("PromoteSynackTargetHandler: Error checking for existing main target codename '%s' under platform %d: %v", mainTargetCodename, req.PlatformID, err)
		http.Error(w, "Internal server error checking for duplicate main target.", http.StatusInternalServerError)
		return
	}
	if err == nil {
		logger.Error("PromoteSynackTargetHandler: A target with codename '%s' already exists for platform ID %d.", mainTargetCodename, req.PlatformID)
		http.Error(w, fmt.Sprintf("A target with codename '%s' already exists for this platform.", mainTargetCodename), http.StatusConflict)
		return
	}

	mainTargetSlug := slugify(mainTargetCodename)
	var existingSlugID int64
	err = database.DB.QueryRow("SELECT id FROM targets WHERE slug = ?", mainTargetSlug).Scan(&existingSlugID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("PromoteSynackTargetHandler: Error checking for existing slug '%s': %v", mainTargetSlug, err)
		http.Error(w, "Internal server error checking slug.", http.StatusInternalServerError)
		return
	}
	if err == nil {
		mainTargetSlug = fmt.Sprintf("%s-%s", mainTargetSlug, uuid.New().String()[:4])
	}

	tx, err := database.DB.Begin()
	if err != nil {
		logger.Error("PromoteSynackTargetHandler: Error beginning transaction: %v", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	targetStmt, err := tx.Prepare(`INSERT INTO targets (platform_id, slug, codename, link, notes) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		logger.Error("PromoteSynackTargetHandler: Error preparing target insert: %v", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	res, err := targetStmt.Exec(req.PlatformID, mainTargetSlug, mainTargetCodename, mainTargetLink, mainTargetNotes)
	if err != nil {
		tx.Rollback()
		targetStmt.Close()
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			logger.Error("PromoteSynackTargetHandler: Insert failed due to unique constraint (platform+codename or slug): %v", err)
			http.Error(w, "Target insert failed due to unique constraint (platform+codename or slug).", http.StatusConflict)
		} else {
			logger.Error("PromoteSynackTargetHandler: Error inserting new main target: %v", err)
			http.Error(w, "Internal server error inserting target.", http.StatusInternalServerError)
		}
		return
	}
	targetStmt.Close()

	newMainTargetID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		logger.Error("PromoteSynackTargetHandler: Error getting last insert ID for new main target: %v", err)
		http.Error(w, "Internal server error after target insert.", http.StatusInternalServerError)
		return
	}

	createdScopeRules := []models.ScopeRule{}
	allScopeItems := append(req.InScopeItemsOverride, req.OutOfScopeItemsOverride...)
	isInScopeFlag := true

	if len(allScopeItems) > 0 {
		scopeStmt, errPrepScope := tx.Prepare(`INSERT INTO scope_rules 
            (target_id, item_type, pattern, is_in_scope, is_wildcard, description) 
            VALUES (?, ?, ?, ?, ?, ?)`)
		if errPrepScope != nil {
			tx.Rollback()
			logger.Error("PromoteSynackTargetHandler: Error preparing scope_rules insert: %v", errPrepScope)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
			return
		}
		defer scopeStmt.Close()

		for i, item := range allScopeItems {
			if i >= len(req.InScopeItemsOverride) {
				isInScopeFlag = false
			}
			itemType := strings.ToLower(strings.TrimSpace(item.ItemType))
			if itemType == "" {
				itemType = determineItemType(item.Pattern)
			} else if !validScopeItemTypes[itemType] {
				tx.Rollback()
				logger.Error("PromoteSynackTargetHandler: Invalid item_type '%s' for pattern '%s'", item.ItemType, item.Pattern)
				http.Error(w, fmt.Sprintf("Invalid item_type '%s' provided for scope rule.", item.ItemType), http.StatusBadRequest)
				return
			}
			isWildcard := strings.Contains(item.Pattern, "*")
			_, err := scopeStmt.Exec(newMainTargetID, itemType, item.Pattern, isInScopeFlag, isWildcard, item.Description)
			if err != nil {
				tx.Rollback()
				logger.Error("PromoteSynackTargetHandler: Error inserting scope rule '%s' (in_scope: %t): %v", item.Pattern, isInScopeFlag, err)
				http.Error(w, "Internal server error inserting scope rules.", http.StatusInternalServerError)
				return
			}
			createdScopeRules = append(createdScopeRules, models.ScopeRule{TargetID: newMainTargetID, ItemType: itemType, Pattern: item.Pattern, IsInScope: isInScopeFlag, IsWildcard: isWildcard, Description: item.Description})
		}
	}

	_, err = tx.Exec("UPDATE synack_targets SET status = ?, last_seen_timestamp = ? WHERE synack_target_id_str = ?",
		"promoted", time.Now().UTC().Format(time.RFC3339), req.SynackTargetIDStr)
	if err != nil {
		tx.Rollback()
		logger.Error("PromoteSynackTargetHandler: Error updating status for Synack target '%s': %v", req.SynackTargetIDStr, err)
		http.Error(w, "Internal server error updating Synack target status.", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		logger.Error("PromoteSynackTargetHandler: Error committing transaction: %v", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	responseTarget := models.Target{
		ID: newMainTargetID, PlatformID: req.PlatformID, Slug: mainTargetSlug, Codename: mainTargetCodename,
		Link: mainTargetLink, Notes: mainTargetNotes, ScopeRules: createdScopeRules,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(responseTarget); err != nil {
		logger.Error("PromoteSynackTargetHandler: Error encoding response: %v", err)
	}
	logger.Info("Successfully promoted Synack target '%s' to main target ID %d (Codename: '%s')", req.SynackTargetIDStr, newMainTargetID, mainTargetCodename)
}
