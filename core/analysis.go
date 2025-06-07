package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os" // Added for diagnostic printing to stderr
	"sort"
	"strings"
	"toolkit/database"
	"toolkit/logger"

	"github.com/BishopFox/jsluice" // Correct casing
)

// processSlice sorts a slice of strings and removes duplicates.
func processSlice(items []string) []string {
	if len(items) == 0 {
		return items
	}
	sort.Strings(items)
	j := 0
	for i := 1; i < len(items); i++ {
		if items[j] != items[i] {
			j++
			items[j] = items[i]
		}
	}
	return items[:j+1]
}

// AnalyzeJSContent uses jsluice to extract data from JavaScript content.
// It saves discovered URLs to the database and returns the extracted data map.
func AnalyzeJSContent(jsContentBytes []byte, httpLogID int64) (map[string][]string, error) {
	logger.Info("Analyzing JS content for log ID %d (%d bytes)", httpLogID, len(jsContentBytes))
	results := make(map[string][]string)
	analyzer := jsluice.NewAnalyzer(jsContentBytes)

	// --- Fetch TargetID associated with this log entry ---
	var targetID sql.NullInt64
	err := database.DB.QueryRow("SELECT target_id FROM http_traffic_log WHERE id = ?", httpLogID).Scan(&targetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("AnalyzeJSContent: Could not find http_traffic_log entry with ID %d", httpLogID)
		} else {
			logger.Error("AnalyzeJSContent: Error fetching target_id for log ID %d: %v", httpLogID, err)
		}
	}
	logger.Debug("AnalyzeJSContent: Log ID %d is associated with Target ID %v (Valid: %t)", httpLogID, targetID.Int64, targetID.Valid)
	// --- End Fetch TargetID ---

	// Prepare statement for inserting discovered URLs
	stmt, err := database.DB.Prepare(`
		INSERT INTO discovered_urls (target_id, http_traffic_log_id, url, source_type) 
		VALUES (?, ?, ?, ?) 
		ON CONFLICT(http_traffic_log_id, url, source_type) DO NOTHING`)
	if err != nil {
		logger.Error("AnalyzeJSContent: Error preparing discovered_urls insert statement: %v", err)
		// Continue analysis but cannot save URLs
	} else {
		defer stmt.Close()
	}

	// Extract URLs
	urlsFound := []string{}
	for _, urlMatch := range analyzer.GetURLs() {
		if urlMatch == nil {
			continue
		}
		urlStr := strings.TrimSpace(urlMatch.URL)
		if urlStr != "" &&
			!strings.HasPrefix(urlStr, "data:") &&
			!strings.HasPrefix(urlStr, "javascript:") &&
			!strings.HasPrefix(urlStr, "mailto:") &&
			!strings.HasPrefix(urlStr, "tel:") {
			urlsFound = append(urlsFound, urlStr)

			if stmt != nil {
				var targetIDArg interface{}
				if targetID.Valid {
					targetIDArg = targetID.Int64
				}
				_, execErr := stmt.Exec(targetIDArg, httpLogID, urlStr, "jsluice")
				if execErr != nil {
					logger.Error("AnalyzeJSContent: Error inserting discovered URL '%s' for log %d: %v", urlStr, httpLogID, execErr)
				} else {
					logger.Debug("AnalyzeJSContent: Saved discovered URL '%s' for log %d", urlStr, httpLogID)
				}
			}
		}
	}
	if len(urlsFound) > 0 {
		results["URLs"] = processSlice(urlsFound)
	}

	// Extract Secrets - DIAGNOSTIC VERSION
	secrets := []string{}
	logger.Debug("Attempting to get secrets...")
	secretMatches := analyzer.GetSecrets()
	logger.Debug("Found %d potential secret matches.", len(secretMatches))

	for i, secretMatch := range secretMatches {
		if secretMatch == nil {
			logger.Debug("Secret match %d is nil, skipping.", i)
			continue
		}
		// --- DIAGNOSTIC PRINT ---
		// Print the type and the detailed structure of the secretMatch variable to stderr
		fmt.Fprintf(os.Stderr, "[DIAGNOSTIC] Secret Match %d Type: %T\n", i, secretMatch)
		fmt.Fprintf(os.Stderr, "[DIAGNOSTIC] Secret Match %d Value: %+v\n", i, secretMatch)
		// --- END DIAGNOSTIC PRINT ---

		// Comment out the problematic line for now to allow compilation
		// secrets = append(secrets, fmt.Sprintf("Potential Secret: Kind='%s', Value='%s' (Context: %s)", secretMatch.Kind, secretMatch.Value.DecodedString(), secretMatch.Context))
		// secrets = append(secrets, fmt.Sprintf("Potential Secret: Description='%s', Match='%s' (Context: %s)", secretMatch.Description, secretMatch.Match, secretMatch.Context))
		// secrets = append(secrets, fmt.Sprintf("Potential Secret: Kind='%s', Match='%s' (Context: %s)", secretMatch.Finding.Kind, secretMatch.Finding.Match, secretMatch.Context))


		// Replace with a placeholder using the debug output
		secrets = append(secrets, fmt.Sprintf("Raw Secret Match %d: %+v", i, secretMatch))

	}
	if len(secrets) > 0 {
		results["Secrets (Raw Debug)"] = processSlice(secrets) // Change key to indicate debug output
	}

	// TODO: Add extraction for other interesting things jsluice might find
	// Example: DOM XSS Sinks
	// domXSSSinks := []string{}
	// for _, sink := range analyzer.GetDOMXSSSinks() {
	// 	if sink != nil {
	// 		domXSSSinks = append(domXSSSinks, fmt.Sprintf("Sink: %s, Source: %s", sink.Sink, sink.Source))
	// 	}
	// }
	// if len(domXSSSinks) > 0 {
	// 	results["DOM XSS Sinks"] = processSlice(domXSSSinks)
	// }

	if len(results) == 0 {
		logger.Info("No interesting items found by jsluice for log ID %d", httpLogID)
		return nil, fmt.Errorf("no interesting items found")
	}

	// Store processed results in the database
	analysisType := "jsluice"
	resultDataBytes, err := json.Marshal(results)
	if err != nil {
		logger.Error("AnalyzeJSContent: Error marshalling analysis results for log ID %d: %v", httpLogID, err)
		// Proceed to return results even if marshalling/DB store fails, but log the error.
	} else {
		// Delete old results for this log_id and analysis_type, then insert new
		_, delErr := database.DB.Exec("DELETE FROM analysis_results WHERE http_log_id = ? AND analysis_type = ?", httpLogID, analysisType)
		if delErr != nil {
			logger.Error("AnalyzeJSContent: Error deleting old analysis results for log ID %d: %v", httpLogID, delErr)
		}

		_, insErr := database.DB.Exec("INSERT INTO analysis_results (http_log_id, analysis_type, result_data) VALUES (?, ?, ?)", httpLogID, analysisType, string(resultDataBytes))
		if insErr != nil {
			logger.Error("AnalyzeJSContent: Error inserting analysis results for log ID %d: %v", httpLogID, insErr)
		} else {
			logger.Info("AnalyzeJSContent: Successfully stored jsluice analysis results for log ID %d", httpLogID)
		}
	}

	return results, nil
}