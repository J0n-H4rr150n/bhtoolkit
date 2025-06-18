package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// SubdomainDiscoveryRequest defines the expected payload for initiating a subdomain scan.
type SubdomainDiscoveryRequest struct {
	Domain      string   `json:"domain"`                 // The primary domain to scan, usually derived from the target
	SubfinderID string   `json:"subfinder_id,omitempty"` // Optional: Specific subfinder config ID from settings
	Recursive   bool     `json:"recursive,omitempty"`    // Subfinder -r flag
	Sources     []string `json:"sources,omitempty"`      // Subfinder -sources flag (comma-separated string or array)
}

// CreateDomainHandler handles POST requests to create a new domain for a target.
// @Summary Create a new domain
// @Description Adds a new domain/subdomain entry associated with a target.
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain_request body models.Domain true "Domain creation request" SchemaExample({\n  "target_id": 1,\n  "domain_name": "test.example.com",\n  "source": "manual",\n  "is_in_scope": true,\n  "notes": "Initial discovery"\n})
// @Success 201 {object} models.Domain "Successfully created domain"
// @Failure 400 {object} models.ErrorResponse "Invalid request payload or missing fields"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /domains [post]
func CreateDomainHandler(w http.ResponseWriter, r *http.Request) {
	var domain models.Domain
	if err := json.NewDecoder(r.Body).Decode(&domain); err != nil {
		logger.Error("CreateDomainHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if domain.TargetID == 0 || strings.TrimSpace(domain.DomainName) == "" {
		http.Error(w, "target_id and domain_name are required", http.StatusBadRequest)
		return
	}

	id, err := database.CreateDomain(domain)
	if err != nil {
		logger.Error("CreateDomainHandler: Error creating domain: %v", err)
		if strings.Contains(err.Error(), "already exists") {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "Failed to create domain", http.StatusInternalServerError)
		}
		return
	}
	domain.ID = id
	// Timestamps are set by the database function or triggers.
	// Fetch the created domain to get all DB-generated values.
	createdDomain, fetchErr := database.GetDomainByID(id)
	if fetchErr != nil {
		logger.Error("CreateDomainHandler: Error fetching newly created domain %d: %v", id, fetchErr)
		// Fallback to returning the input domain with ID, but log the issue.
		domain.CreatedAt = time.Now() // Approximate
		domain.UpdatedAt = time.Now() // Approximate
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(domain)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdDomain)
}

// GetDomainsHandler handles GET requests to list domains for a target, with pagination and filtering.
// @Summary List domains for a target
// @Description Retrieves a paginated list of domains associated with a target, with filtering and sorting options.
// @Tags Domains
// @Produce json
// @Param target_id path int true "Target ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(25)
// @Param sort_by query string false "Column to sort by (e.g., domain_name, created_at)" default(domain_name)
// @Param sort_order query string false "Sort order (asc, desc)" default(asc)
// @Param domain_name_search query string false "Search term for domain name"
// @Param source_search query string false "Search term for source"
// @Param is_in_scope query boolean false "Filter by in-scope status"
// @Param is_favorite query boolean false "Filter by favorite status"
// @Success 200 {object} models.PaginatedDomainsResponse "Successfully retrieved domains"
// @Failure 400 {object} models.ErrorResponse "Invalid target_id or query parameters"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /targets/{target_id}/domains [get]
func GetDomainsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	filters := models.DomainFilters{TargetID: targetID}
	filters.Page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if filters.Page <= 0 {
		filters.Page = 1
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		filters.Limit = 25
	} else {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			filters.Limit = 25
		} else {
			if parsedLimit == 0 {
				filters.Limit = 0
			} else if parsedLimit < 0 || parsedLimit > 200 {
				filters.Limit = 200
			} else {
				filters.Limit = parsedLimit
			}
		}
	}

	filters.SortBy = r.URL.Query().Get("sort_by")
	if filters.SortBy == "" {
		filters.SortBy = "domain_name"
	}
	filters.SortOrder = strings.ToUpper(r.URL.Query().Get("sort_order"))
	if filters.SortOrder != "ASC" && filters.SortOrder != "DESC" {
		filters.SortOrder = "ASC"
	}

	filters.DomainNameSearch = r.URL.Query().Get("domain_name_search")
	filters.SourceSearch = r.URL.Query().Get("source_search")

	if isInScopeStr := r.URL.Query().Get("is_in_scope"); isInScopeStr != "" {
		isInScopeVal, err := strconv.ParseBool(isInScopeStr)
		if err == nil {
			filters.IsInScope = &isInScopeVal
		}
	}

	if isFavoriteStr := r.URL.Query().Get("is_favorite"); isFavoriteStr != "" {
		isFavoriteVal, err := strconv.ParseBool(isFavoriteStr)
		if err == nil {
			filters.IsFavorite = &isFavoriteVal
		}
	}

	// Filter by httpx scan status
	filters.HttpxScanStatus = r.URL.Query().Get("httpx_scan_status")
	if filters.HttpxScanStatus == "" {
		filters.HttpxScanStatus = "all" // Default if not provided
	}

	// New distinct filters
	filters.FilterHTTPStatusCode = r.URL.Query().Get("filter_http_status_code")
	filters.FilterHTTPServer = r.URL.Query().Get("filter_http_server")
	filters.FilterHTTPTech = r.URL.Query().Get("filter_http_tech")
	domains, totalRecords, distinctValues, err := database.GetDomains(filters)
	if err != nil {
		logger.Error("GetDomainsHandler: Error getting domains for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve domains", http.StatusInternalServerError)
		return
	}

	var totalPages int64
	if filters.Limit > 0 {
		totalPages = (totalRecords + int64(filters.Limit) - 1) / int64(filters.Limit)
	} else {
		if totalRecords > 0 {
			totalPages = 1
		} else {
			totalPages = 0
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := models.PaginatedDomainsResponse{
		Page:         filters.Page,
		Limit:        filters.Limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
		SortBy:       filters.SortBy,
		SortOrder:    filters.SortOrder,
		Records:      domains,
	}
	if distinctValues != nil { // Populate distinct values if returned
		response.DistinctHttpStatusCodes = distinctValues.DistinctHttpStatusCodes
		response.DistinctHttpServers = distinctValues.DistinctHttpServers
		response.DistinctHttpTechs = distinctValues.DistinctHttpTechs
	}
	json.NewEncoder(w).Encode(response)
}

// UpdateDomainHandler handles PUT requests to update an existing domain.
// @Summary Update an existing domain
// @Description Updates details of a specific domain by its ID.
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Param domain_update_request body models.Domain true "Domain update request" SchemaExample({\n  "is_in_scope": false,\n  "notes": "Out of scope based on new info"\n})
// @Success 200 {object} models.Domain "Successfully updated domain"
// @Failure 400 {object} models.ErrorResponse "Invalid request payload or domain_id"
// @Failure 404 {object} models.ErrorResponse "Domain not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /domains/{domain_id} [put]
func UpdateDomainHandler(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domain_id")
	domainID, err := strconv.ParseInt(domainIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid domain_id in path", http.StatusBadRequest)
		return
	}

	var domainUpdates models.Domain
	if err := json.NewDecoder(r.Body).Decode(&domainUpdates); err != nil {
		logger.Error("UpdateDomainHandler: Error decoding request body for domain %d: %v", domainID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	domainUpdates.ID = domainID

	err = database.UpdateDomain(domainUpdates)
	if err != nil {
		logger.Error("UpdateDomainHandler: Error updating domain %d: %v", domainID, err)
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Domain not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update domain", http.StatusInternalServerError)
		}
		return
	}

	updatedDomain, fetchErr := database.GetDomainByID(domainID)
	if fetchErr != nil {
		logger.Error("UpdateDomainHandler: Error fetching updated domain %d: %v", domainID, fetchErr)
		// Fallback to returning the input data with an approximate timestamp.
		domainUpdates.UpdatedAt = time.Now()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(domainUpdates)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedDomain)
}

// DeleteDomainHandler handles DELETE requests to delete a domain.
// @Summary Delete a domain
// @Description Deletes a specific domain by its ID.
// @Tags Domains
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Success 204 "Successfully deleted domain"
// @Failure 400 {object} models.ErrorResponse "Invalid domain_id"
// @Failure 404 {object} models.ErrorResponse "Domain not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /domains/{domain_id} [delete]
func DeleteDomainHandler(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domain_id")
	domainID, err := strconv.ParseInt(domainIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid domain_id in path", http.StatusBadRequest)
		return
	}

	err = database.DeleteDomain(domainID)
	if err != nil {
		logger.Error("DeleteDomainHandler: Error deleting domain %d: %v", domainID, err)
		if strings.Contains(err.Error(), "not found") { // Check if the DB layer indicates "not found"
			http.Error(w, "Domain not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete domain", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DiscoverSubdomainsHandler handles POST requests to initiate subdomain discovery for a target.
// @Summary Discover subdomains for a target
// @Description Initiates a subdomain discovery process (e.g., using subfinder) for the specified target. This is an asynchronous operation.
// @Tags Domains
// @Accept json
// @Produce json
// @Param target_id path int true "Target ID"
// @Param discovery_request body SubdomainDiscoveryRequest true "Subdomain discovery options" SchemaExample({\n  "domain": "example.com",\n  "recursive": true\n})
// @Success 202 {object} map[string]string "Discovery process initiated"
// @Failure 400 {object} models.ErrorResponse "Invalid request payload or target_id"
// @Failure 500 {object} models.ErrorResponse "Internal server error or subfinder not configured"
// @Router /targets/{target_id}/domains/discover [post]
func DiscoverSubdomainsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	var req SubdomainDiscoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("DiscoverSubdomainsHandler: Error decoding request body for target %d: %v", targetID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(req.Domain) == "" {
		http.Error(w, "Domain is required in the request payload", http.StatusBadRequest)
		return
	}

	_, err = exec.LookPath("subfinder")
	if err != nil {
		logger.Error("DiscoverSubdomainsHandler: subfinder command not found in PATH: %v", err)
		http.Error(w, "Subdomain discovery tool (subfinder) is not configured or not found.", http.StatusInternalServerError)
		return
	}

	go runSubfinderAndStoreResults(targetID, req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message":   "Subdomain discovery process initiated for " + req.Domain,
		"target_id": targetIDStr,
	})
}

func runSubfinderAndStoreResults(targetID int64, config SubdomainDiscoveryRequest) {
	logger.Info("Starting subfinder for target %d, domain %s", targetID, config.Domain)

	args := []string{"-d", config.Domain, "-json", "-silent"}
	if config.Recursive {
		args = append(args, "-r")
	}
	if len(config.Sources) > 0 {
		args = append(args, "-sources", strings.Join(config.Sources, ","))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "subfinder", args...)
	output, err := cmd.Output()

	if ctx.Err() == context.DeadlineExceeded {
		logger.Error("Subfinder command timed out for target %d, domain %s", targetID, config.Domain)
		return
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			logger.Error("Subfinder execution failed for target %d, domain %s. ExitError: %v. Stderr: %s", targetID, config.Domain, err, string(exitErr.Stderr))
		} else {
			logger.Error("Subfinder execution failed for target %d, domain %s: %v", targetID, config.Domain, err)
		}
		return
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var discoveredCount int
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var result struct {
			Host string `json:"host"`
		}
		if err := json.Unmarshal([]byte(line), &result); err == nil && result.Host != "" {
			domainEntry := models.Domain{
				TargetID:   targetID,
				DomainName: result.Host,
				Source:     models.NullString("subfinder"),
				IsInScope:  false,
			}
			_, createErr := database.CreateDomain(domainEntry)
			if createErr != nil {
				if !strings.Contains(createErr.Error(), "already exists") {
					logger.Error("Failed to store subdomain '%s' for target %d: %v", result.Host, targetID, createErr)
				}
			} else {
				discoveredCount++
			}
		} else if err != nil {
			logger.Warn("Failed to parse subfinder output line: '%s'. Error: %v", line, err)
		}
	}
	logger.Info("Subfinder finished for target %d, domain %s. Discovered and attempted to store %d new subdomains.", targetID, config.Domain, discoveredCount)
}

// ImportInScopeDomainsHandler handles POST requests to import in-scope domains from a target's scope rules.
// @Summary Import in-scope domains
// @Description Imports domains/subdomains from the target's 'in-scope' rules into the domains table.
// @Tags Domains
// @Produce json
// @Param target_id path int true "Target ID"
// @Success 200 {object} map[string]interface{} "Import summary (imported_count, skipped_count, message)"
// @Failure 400 {object} models.ErrorResponse "Invalid target_id"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /targets/{target_id}/domains/import-scope [post]
func ImportInScopeDomainsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	scopeRules, err := database.GetScopeRulesByTargetID(targetID)
	if err != nil {
		logger.Error("ImportInScopeDomainsHandler: Error fetching scope rules for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve scope rules", http.StatusInternalServerError)
		return
	}

	var importedCount, skippedCount int
	var errorMessages []string

	for _, rule := range scopeRules {
		if rule.IsInScope && (rule.ItemType == "domain" || rule.ItemType == "subdomain") {
			originalPattern := strings.TrimSpace(rule.Pattern)
			if originalPattern == "" {
				continue
			}

			// Default values
			domainToStore := "" // Initialize to empty; will be set if rule is importable
			isWildcard := false
			notesForDomain := models.NullString(rule.Description) // Default to rule's description

			if strings.HasPrefix(originalPattern, "*.") {
				domainToStore = strings.TrimPrefix(originalPattern, "*.")
				isWildcard = true
				notesForDomain = models.NullString(fmt.Sprintf("Imported from wildcard scope: %s", originalPattern))
			} else {
				// Attempt to simplify regex-like patterns
				cleanedPattern := originalPattern
				isPotentiallySimpleRegex := strings.HasPrefix(cleanedPattern, "^") && strings.HasSuffix(cleanedPattern, "$")

				if isPotentiallySimpleRegex {
					cleanedPattern = strings.TrimPrefix(cleanedPattern, "^")
					cleanedPattern = strings.TrimSuffix(cleanedPattern, "$")
				}
				cleanedPattern = strings.ReplaceAll(cleanedPattern, "\\.", ".")

				if strings.HasPrefix(cleanedPattern, ".+") {
					domainPart := strings.TrimPrefix(cleanedPattern, ".+")
					if strings.HasPrefix(domainPart, ".") {
						domainToStore = strings.TrimPrefix(domainPart, ".")
						notesForDomain = models.NullString(fmt.Sprintf("Imported from regex scope: %s (derived: %s)", originalPattern, domainToStore))
					} else { // Should ideally have a dot, but handle if not
						domainToStore = domainPart
						notesForDomain = models.NullString(fmt.Sprintf("Imported from regex scope: %s (derived: %s)", originalPattern, domainToStore))
					}
				} else if !strings.ContainsAny(cleanedPattern, "^$+*?()[]{}|\\") {
					// If no complex regex characters remain after basic cleaning
					domainToStore = cleanedPattern
					if originalPattern != cleanedPattern { // If simplification occurred
						notesForDomain = models.NullString(fmt.Sprintf("Imported from regex-like scope: %s (simplified: %s)", originalPattern, domainToStore))
					}
				} else if strings.ContainsAny(originalPattern, "^$+*?()[]{}|\\") {
					logger.Info("ImportInScopeDomainsHandler: Skipping import of complex regex scope rule '%s' into domains table for target %d.", originalPattern, targetID)
					continue // Skip this rule
				}
			}
			if domainToStore == "" && !isWildcard { // If not a wildcard and not simplified to a domain
				if !strings.ContainsAny(originalPattern, "^$+*?()[]{}|\\") { // If it was a plain domain initially
					domainToStore = originalPattern // Use original pattern if it was plain
				} else {
					// This case should ideally be caught by the complex regex skip, but as a fallback:
					logger.Info("ImportInScopeDomainsHandler: Pattern '%s' for target %d was not converted to a simple domain and is not a wildcard; skipping.", originalPattern, targetID)
					continue
				}
			}
			if strings.TrimSpace(domainToStore) == "" {
				logger.Info("ImportInScopeDomainsHandler: Derived domain to store is empty for original pattern '%s', target %d; skipping.", originalPattern, targetID)
				continue
			}
			domainEntry := models.Domain{
				TargetID:        targetID,
				DomainName:      domainToStore,
				Source:          models.NullString("scope_import"),
				IsInScope:       true,
				IsWildcardScope: isWildcard,
				Notes:           notesForDomain,
			}
			_, createErr := database.CreateDomain(domainEntry)
			if createErr != nil {
				if strings.Contains(createErr.Error(), "already exists") {
					skippedCount++
				} else {
					logger.Error("ImportInScopeDomainsHandler: Failed to import domain '%s' (from pattern '%s') for target %d: %v", domainToStore, originalPattern, targetID, createErr)
					errorMessages = append(errorMessages, fmt.Sprintf("Error importing '%s' from pattern '%s': %v", domainToStore, originalPattern, createErr))
				}
			} else {
				importedCount++
			}
		}
	}

	message := fmt.Sprintf("Import complete. Imported %d new domains, skipped %d (already exist).", importedCount, skippedCount)
	if len(errorMessages) > 0 {
		message += " Errors: " + strings.Join(errorMessages, "; ")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        message,
		"imported_count": importedCount,
		"skipped_count":  skippedCount,
	})
	logger.Info("ImportInScopeDomainsHandler: For target %d, imported %d, skipped %d domains. Errors: %d", targetID, importedCount, skippedCount, len(errorMessages))
}

// DeleteAllDomainsForTargetHandler handles DELETE requests to remove all domains for a specific target.
// @Summary Delete all domains for a target
// @Description Deletes all domain entries associated with the specified target ID.
// @Tags Domains
// @Produce json
// @Param target_id path int true "Target ID"
// @Success 200 {object} map[string]interface{} "Deletion summary (deleted_count, message)"
// @Failure 400 {object} models.ErrorResponse "Invalid target_id"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /targets/{target_id}/domains/all [delete]
func DeleteAllDomainsForTargetHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	deletedCount, err := database.DeleteAllDomainsForTarget(targetID)
	if err != nil {
		logger.Error("DeleteAllDomainsForTargetHandler: Error deleting domains for target %d: %v", targetID, err)
		http.Error(w, "Failed to delete domains for target", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       fmt.Sprintf("Successfully deleted %d domains for target ID %d.", deletedCount, targetID),
		"deleted_count": deletedCount,
	})
	logger.Info("DeleteAllDomainsForTargetHandler: Deleted %d domains for target ID %d", deletedCount, targetID)
}

// GetDomainDetailHandler handles GET requests for details of a specific domain.
// @Summary Get domain details
// @Description Retrieves detailed information for a specific domain, including httpx results.
// @Tags Domains
// @Produce json
// @Param domain_id path int true "Domain ID"
// @Success 200 {object} models.Domain "Successfully retrieved domain details"
// @Failure 400 {object} models.ErrorResponse "Invalid domain_id"
// @Failure 404 {object} models.ErrorResponse "Domain not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /domains/{domain_id}/details [get]
func GetDomainDetailHandler(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domain_id")
	domainID, err := strconv.ParseInt(domainIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid domain_id in path", http.StatusBadRequest)
		return
	}

	domain, err := database.GetDomainByID(domainID) // This function will need to be updated
	if err != nil {
		// GetDomainByID should return a specific error for "not found"
		http.Error(w, err.Error(), http.StatusNotFound) // Or InternalServerError based on error type
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(domain)
}

// RunHttpxForDomainsHandler handles POST requests to run httpx against selected domains.
// @Summary Run httpx against selected domains
// @Description Initiates an asynchronous httpx scan for a list of domain IDs associated with a target.
// @Tags Domains
// @Accept json
// @Produce json
// @Param target_id path int true "Target ID"
// @Param domain_ids_request body object true "List of domain IDs" SchemaExample({"domain_ids": [1, 5, 10]})
// @Success 202 {object} map[string]string "Httpx scan initiated"
// @Failure 400 {object} models.ErrorResponse "Invalid request payload or target_id"
// @Failure 500 {object} models.ErrorResponse "Internal server error or httpx not configured"
// @Router /targets/{target_id}/domains/run-httpx [post]
func RunHttpxForDomainsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	var req struct {
		DomainIDs []int64 `json:"domain_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("RunHttpxForDomainsHandler: Error decoding request body for target %d: %v", targetID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(req.DomainIDs) == 0 {
		http.Error(w, "No domain IDs provided", http.StatusBadRequest)
		return
	}

	// Fetch domain names from the database based on IDs
	domains, err := database.GetDomainsByIDs(req.DomainIDs)
	if err != nil {
		logger.Error("RunHttpxForDomainsHandler: Error fetching domains by IDs for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve domain details", http.StatusInternalServerError)
		return
	}

	go RunHttpxScan(targetID, domains) // Run asynchronously

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message":   fmt.Sprintf("Httpx scan initiated for %d domain(s).", len(domains)),
		"target_id": targetIDStr,
	})
}

// SetDomainFavoriteHandler handles requests to set the favorite status of a domain.
// @Summary Set Domain Favorite Status
// @Description Sets or unsets the favorite status of a specific domain.
// @Tags Domains
// @Accept json
// @Produce json
// @Param domain_id path int true "ID of the domain"
// @Param favorite_request body models.SetFavoriteRequest true "Favorite status" SchemaExample({\n  "is_favorite": true\n})
// @Success 200 {object} map[string]string "message: Favorite status updated successfully."
// @Failure 400 {object} models.ErrorResponse "Invalid domain_id or request body"
// @Failure 404 {object} models.ErrorResponse "Domain not found"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /domains/{domain_id}/favorite [put]
func SetDomainFavoriteHandler(w http.ResponseWriter, r *http.Request) {
	domainIDStr := chi.URLParam(r, "domain_id")
	domainID, err := strconv.ParseInt(domainIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid domain ID", http.StatusBadRequest)
		return
	}

	var reqBody struct {
		IsFavorite bool `json:"is_favorite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := database.SetDomainFavoriteStatus(domainID, reqBody.IsFavorite); err != nil {
		// database.SetDomainFavoriteStatus logs specific errors and can return a "not found" error
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Domain not found: "+err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update favorite status: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Favorite status updated successfully."})
}

// FavoriteAllFilteredDomainsHandler handles requests to mark all domains matching filters as favorite.
// @Summary Favorite all filtered domains
// @Description Marks all domains matching the provided filters for a target as favorite.
// @Tags Domains
// @Accept json
// @Produce json
// @Param target_id path int true "Target ID"
// @Param filters body models.DomainFilters true "Filter criteria" SchemaExample({\n  "domain_name_search": "b2b",\n  "source_search": "subfinder",\n  "is_in_scope": true\n})
// @Success 200 {object} map[string]interface{} "message: Domains updated, updated_count: X"
// @Failure 400 {object} models.ErrorResponse "Invalid target_id or request body"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /targets/{target_id}/domains/favorite-filtered [post]
func FavoriteAllFilteredDomainsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	var filters struct {
		DomainNameSearch string `json:"domain_name_search"`
		SourceSearch     string `json:"source_search"`
		IsInScope        *bool  `json:"is_in_scope"`
	}

	if err := json.NewDecoder(r.Body).Decode(&filters); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	updatedCount, err := database.FavoriteAllFilteredDomainsDB(targetID, filters.DomainNameSearch, filters.SourceSearch, filters.IsInScope)
	if err != nil {
		logger.Error("FavoriteAllFilteredDomainsHandler: Error updating domains for target %d: %v", targetID, err)
		http.Error(w, "Failed to update domains: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"message": "Domains updated successfully.", "updated_count": updatedCount})
}

// RunHttpxForAllFilteredDomainsHandler handles POST requests to run httpx against all domains matching filter criteria for a target.
// @Summary Run httpx against all filtered domains
// @Description Initiates an asynchronous httpx scan for all domains matching the provided filters for a target.
// @Tags Domains
// @Accept json
// @Produce json
// @Param target_id path int true "Target ID"
// @Param filters body models.DomainFilters true "Filter criteria (domain_name_search, source_search, is_in_scope, is_favorite)"
// @Success 202 {object} map[string]string "Httpx scan initiated for all filtered domains"
// @Failure 400 {object} models.ErrorResponse "Invalid request payload or target_id"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /targets/{target_id}/domains/run-httpx-all-filtered [post]
func RunHttpxForAllFilteredDomainsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	var filters models.DomainFilters
	if err := json.NewDecoder(r.Body).Decode(&filters); err != nil {
		logger.Error("RunHttpxForAllFilteredDomainsHandler: Error decoding request body for target %d: %v", targetID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	filters.TargetID = targetID // Ensure TargetID from path is used

	// Get all domain IDs matching filters (no pagination)
	// This new DB function will be created next.
	domainIDs, err := database.GetDomainIDsByFilters(filters)
	if err != nil {
		logger.Error("RunHttpxForAllFilteredDomainsHandler: Error fetching domain IDs by filters for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve domain IDs for scan", http.StatusInternalServerError)
		return
	}

	if len(domainIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Or 202, but 200 is fine if no action is taken.
		json.NewEncoder(w).Encode(map[string]string{"message": "No domains found matching the filters. Httpx scan not initiated."})
		return
	}

	// Fetch full domain objects for these IDs
	domainsToScan, err := database.GetDomainsByIDs(domainIDs)
	if err != nil {
		logger.Error("RunHttpxForAllFilteredDomainsHandler: Error fetching full domain details by IDs for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve full domain details for scan", http.StatusInternalServerError)
		return
	}

	go RunHttpxScan(targetID, domainsToScan) // Run asynchronously

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"message":   fmt.Sprintf("Httpx scan initiated for %d filtered domain(s).", len(domainsToScan)),
		"target_id": targetIDStr,
	})
}
