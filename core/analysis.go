package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath" // For cleaning paths
	"regexp"
	"sort"
	"strings"
	"toolkit/database"
	"toolkit/logger"

	"github.com/BishopFox/jsluice" // Correct casing
)

var (
	// Regexes for AnalyzeJSContent
	// Define a regex to capture Angular routerLink paths
	// This regex looks for routerLink="...", routerLink='...', or [routerLink]="..."
	routerLinkRegexGlobal = regexp.MustCompile(`(?:routerLink|\[routerLink\])\s*=\s*["'](/[^"']+)["']`)
	// Regex for general interesting string literals that look like paths
	generalPathRegexGlobal = regexp.MustCompile(`["'](/[\w\/-]{3,})["']`)
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
	// Convert jsContent to string for custom regex operations
	jsContentStr := string(jsContentBytes)

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
		// Allow only http and https schemes
		if urlStr != "" && (strings.HasPrefix(strings.ToLower(urlStr), "http:") || strings.HasPrefix(strings.ToLower(urlStr), "https:")) {
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

	// Extract Secrets
	secrets := []string{}
	secretMatches := analyzer.GetSecrets()
	for _, secretMatch := range secretMatches {
		if secretMatch == nil {
			continue
		}
		// Construct a descriptive string for the secret
		var secretDescParts []string
		if secretMatch.Kind != "" {
			secretDescParts = append(secretDescParts, fmt.Sprintf("Kind: %s", secretMatch.Kind))
		}
		// The actual matched data is in the 'Data' field.
		// It's of type 'any', so we'll format it as a string.
		// Specific matchers might put a string directly, or a map.
		if secretMatch.Data != nil {
			secretDescParts = append(secretDescParts, fmt.Sprintf("Data: %v", secretMatch.Data))
		}

		// Context is also 'any'
		if secretMatch.Context != nil {
			// Truncate long contexts for display
			contextDisplay := fmt.Sprintf("%v", secretMatch.Context) // Convert 'any' to string
			if len(contextDisplay) > 100 {
				contextDisplay = contextDisplay[:97] + "..."
			}
			secretDescParts = append(secretDescParts, fmt.Sprintf("Context: %s", contextDisplay))
		}
		if len(secretDescParts) > 0 { // Only add if we have some parts
			secrets = append(secrets, strings.Join(secretDescParts, ", "))
		}
	}
	if len(secrets) > 0 {
		results["Secrets"] = processSlice(secrets)
	}

	// Extract Generic Strings (can be noisy, but might catch other things)
	var genericStrings []string
	// Use a tree-sitter query to find all string literals
	analyzer.Query("(string) @string", func(n *jsluice.Node) {
		// n.RawString() gets the content of the string literal, including quotes.
		// n.DecodedString() attempts to decode escape sequences.
		// For generic strings, DecodedString is usually what we want.
		genericStrings = append(genericStrings, n.DecodedString())
	})
	if len(genericStrings) > 0 {
		results["Generic Strings"] = processSlice(genericStrings)
	}

	// Apply custom regex for Angular routerLink
	routerLinkMatches := routerLinkRegexGlobal.FindAllStringSubmatch(jsContentStr, -1)
	var foundRouterLinks []string
	for _, match := range routerLinkMatches {
		if len(match) > 1 && match[1] != "" {
			cleanedPath := filepath.Clean(match[1])
			if cleanedPath != "/" && !strings.HasPrefix(cleanedPath, "/") {
				cleanedPath = "/" + cleanedPath
			}
			foundRouterLinks = append(foundRouterLinks, cleanedPath)
		}
	}
	if len(foundRouterLinks) > 0 {
		results["Angular RouterLinks (Regex)"] = processSlice(foundRouterLinks)
	}

	// Apply custom regex for general path-like strings
	generalPathMatches := generalPathRegexGlobal.FindAllStringSubmatch(jsContentStr, -1)
	var foundGeneralPaths []string
	for _, match := range generalPathMatches {
		if len(match) > 1 && match[1] != "" {
			foundGeneralPaths = append(foundGeneralPaths, filepath.Clean(match[1]))
		}
	}
	if len(foundGeneralPaths) > 0 {
		results["Potential Paths (Regex)"] = processSlice(foundGeneralPaths)
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
