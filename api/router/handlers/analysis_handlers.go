package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"toolkit/core"
	"toolkit/database"
	"toolkit/logger"
)

// AnalyzeJavaScriptRequest defines the expected structure for the request body
// for the AnalyzeJSLinksHandler.
type AnalyzeJavaScriptRequest struct {
	HTTPLogID int64 `json:"http_log_id" binding:"required"`
}

// AnalyzeJavaScriptResponse defines the structure for the analysis results
// returned by the AnalyzeJSLinksHandler.
type AnalyzeJavaScriptResponse struct {
	LogID   int64               `json:"log_id"`
	Results map[string][]string `json:"results"`
	Message string              `json:"message,omitempty"`
}

// AnalyzeJSLinksHandler handles requests to analyze JavaScript content from a logged HTTP response.
// It fetches the response body for a given http_log_id, checks if it's JavaScript or JSON,
// runs the analysis using core.AnalyzeJSContent, and returns the extracted items.
func AnalyzeJSLinksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("AnalyzeJSLinksHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AnalyzeJavaScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("AnalyzeJSLinksHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.HTTPLogID == 0 {
		logger.Error("AnalyzeJSLinksHandler: http_log_id is required")
		http.Error(w, "http_log_id is required", http.StatusBadRequest)
		return
	}

	var resBodyBytes []byte
	var contentType sql.NullString
	query := `SELECT response_body, response_content_type FROM http_traffic_log WHERE id = ?`
	err := database.DB.QueryRow(query, req.HTTPLogID).Scan(&resBodyBytes, &contentType)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("AnalyzeJSLinksHandler: Log entry with ID %d not found", req.HTTPLogID)
			http.Error(w, fmt.Sprintf("Log entry with ID %d not found", req.HTTPLogID), http.StatusNotFound)
		} else {
			logger.Error("AnalyzeJSLinksHandler: Error querying log ID %d: %v", req.HTTPLogID, err)
			http.Error(w, "Error retrieving log entry from database", http.StatusInternalServerError)
		}
		return
	}

	isAnalyzable := false
	if contentType.Valid {
		ctLower := strings.ToLower(contentType.String)
		if strings.Contains(ctLower, "javascript") || strings.Contains(ctLower, "ecmascript") || strings.Contains(ctLower, "json") {
			isAnalyzable = true
		}
	}

	if !isAnalyzable || len(resBodyBytes) == 0 {
		msg := "Response body is empty or not identified as JavaScript/JSON. Analysis skipped."
		if len(resBodyBytes) > 0 && !isAnalyzable {
			msg = fmt.Sprintf("Content-Type '%s' does not appear to be JavaScript/JSON. Analysis skipped.", contentType.String)
		} else if len(resBodyBytes) == 0 {
			msg = "Response body is empty. Analysis skipped."
		}
		logger.Info("AnalyzeJSLinksHandler: %s (Log ID %d)", msg, req.HTTPLogID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AnalyzeJavaScriptResponse{LogID: req.HTTPLogID, Results: make(map[string][]string), Message: msg})
		return
	}

	logger.Info("AnalyzeJSLinksHandler: Analyzing JS/JSON content for log ID %d", req.HTTPLogID)
	results, analysisErr := core.AnalyzeJSContent(resBodyBytes, req.HTTPLogID)

	if analysisErr != nil {
		logger.Error("AnalyzeJSLinksHandler: Analysis failed for log ID %d: %v", req.HTTPLogID, analysisErr)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(AnalyzeJavaScriptResponse{LogID: req.HTTPLogID, Results: make(map[string][]string), Message: fmt.Sprintf("Analysis encountered an error: %v", analysisErr)})
		return
	}

	logger.Info("AnalyzeJSLinksHandler: Analysis complete for log ID %d. Found %d categories of results.", req.HTTPLogID, len(results))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(AnalyzeJavaScriptResponse{LogID: req.HTTPLogID, Results: results})
}
