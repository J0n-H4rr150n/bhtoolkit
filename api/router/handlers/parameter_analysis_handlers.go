package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// Helper function to extract parameter keys from a URL string
func getParamKeysFromURL(rawURL string) ([]string, string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", err
	}
	params := parsedURL.Query()
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Ensure consistent order
	return keys, parsedURL.Path, nil
}

// AnalyzeTargetForParameterizedURLsHandler triggers analysis of a target's proxy logs
// to find and store unique parameterized URLs.
func AnalyzeTargetForParameterizedURLsHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("AnalyzeTargetForParameterizedURLsHandler: Invalid target_id: %v", err)
		http.Error(w, "Invalid target_id", http.StatusBadRequest)
		return
	}

	// Fetch all HTTP logs for the target. For very large logs, consider batching.
	// Using a large limit for now, similar to the previous client-side approach.
	// A more robust solution might involve background processing or streaming.
	logs, _, err := database.GetHTTPTrafficLogEntries(models.ProxyLogFilters{
		TargetID:  targetID,
		Page:      1,
		Limit:     100000, // A large limit to process all logs; adjust as needed
		SortBy:    "id",   // Or timestamp
		SortOrder: "ASC",
	})
	if err != nil {
		logger.Error("AnalyzeTargetForParameterizedURLsHandler: Error fetching logs for target %d: %v", targetID, err)
		http.Error(w, "Failed to fetch logs for analysis", http.StatusInternalServerError)
		return
	}

	var processedCount, newEntriesCount int
	for _, log := range logs {
		if !log.RequestURL.Valid || !strings.Contains(log.RequestURL.String, "?") {
			continue // Skip URLs without query parameters
		}

		paramKeys, requestPath, err := getParamKeysFromURL(log.RequestURL.String)
		if err != nil {
			logger.Info("AnalyzeTargetForParameterizedURLsHandler: Could not parse URL '%s' from log ID %d: %v (Skipping)", log.RequestURL.String, log.ID, err) // Changed to Info if Warn is not available
			continue
		}

		if len(paramKeys) == 0 {
			continue // Should not happen if '?' is present, but good check
		}

		pURL := models.ParameterizedURL{
			TargetID:         sql.NullInt64{Int64: targetID, Valid: true},
			HTTPTrafficLogID: log.ID,
			RequestMethod:    log.RequestMethod, // log.RequestMethod is now sql.NullString
			RequestPath:      models.NullString(requestPath),
			ParamKeys:        strings.Join(paramKeys, ","),
			ExampleFullURL:   log.RequestURL, // log.RequestURL is now sql.NullString
			// Notes can be added later via UI
		}

		_, created, err := database.CreateOrUpdateParameterizedURL(pURL)
		if err != nil {
			logger.Error("AnalyzeTargetForParameterizedURLsHandler: Error saving parameterized URL for log %d: %v", log.ID, err)
			// Continue processing other logs
		} else {
			processedCount++
			if created {
				newEntriesCount++
			}
		}
	}

	response := map[string]interface{}{
		"message":                      "Parameter analysis completed.",
		"total_logs_scanned":           len(logs),
		"parameterized_urls_processed": processedCount,
		"new_unique_entries_found":     newEntriesCount,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logger.Info("Parameter analysis for target %d completed. Scanned: %d, Processed: %d, New: %d", targetID, len(logs), processedCount, newEntriesCount)
}

// GetParameterizedURLsHandler retrieves stored parameterized URLs for a target.
func GetParameterizedURLsHandler(w http.ResponseWriter, r *http.Request) {
	var params models.ParameterizedURLFilters

	targetIDStr := r.URL.Query().Get("target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil || targetID == 0 {
		logger.Error("GetParameterizedURLsHandler: Invalid or missing target_id: %v", err)
		http.Error(w, "Invalid or missing target_id", http.StatusBadRequest)
		return
	}
	params.TargetID = targetID

	params.Page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if params.Page < 1 {
		params.Page = 1
	}
	params.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if params.Limit < 1 || params.Limit > 200 { // Max limit
		params.Limit = 50 // Default limit
	}
	params.SortBy = r.URL.Query().Get("sort_by")
	params.SortOrder = r.URL.Query().Get("sort_order")
	params.RequestMethod = r.URL.Query().Get("request_method")
	params.PathSearch = r.URL.Query().Get("path_search")
	params.ParamKeysSearch = r.URL.Query().Get("param_keys_search")

	urls, totalRecords, err := database.GetParameterizedURLs(params)
	if err != nil {
		logger.Error("GetParameterizedURLsHandler: Error fetching parameterized URLs for target %d: %v", params.TargetID, err)
		http.Error(w, "Failed to retrieve parameterized URLs", http.StatusInternalServerError)
		return
	}

	response := models.PaginatedResponse{
		Page:         params.Page,
		Limit:        params.Limit,
		TotalRecords: totalRecords,
		TotalPages:   (totalRecords + params.Limit - 1) / params.Limit,
		Records:      urls,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
