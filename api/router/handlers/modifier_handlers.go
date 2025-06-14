package handlers

import (
	"compress/gzip"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors" // Import the errors package
	"fmt"
	"io"
	"net" // Added for IP address parsing and checks
	"net/http"
	"net/url" // Added for URL parsing in SSRF check
	"strconv"
	"strings"
	"time"
	"toolkit/config" // Added for accessing app configuration
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"
)

// executeModifiedRequestPayload defines the structure for the incoming request from the frontend.
type executeModifiedRequestPayload struct {
	TaskID  *int64 `json:"task_id,omitempty"` // Optional, for future use (e.g., versioning)
	Method  string `json:"method"`
	URL     string `json:"url"`
	Headers string `json:"headers"` // Raw string of headers, e.g., "Key1: Value1\nKey2: Value2"
	Body    string `json:"body"`    // Raw string of the request body
}

// executeModifiedResponsePayload defines the structure for the response sent back to the frontend.
type executeModifiedResponsePayload struct {
	StatusCode int                 `json:"status_code"`
	StatusText string              `json:"status_text"` // e.g., "200 OK"
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"` // Base64 encoded response body
	Error      string              `json:"error,omitempty"`
	DurationMs int64               `json:"duration_ms"`
}

// AddModifierTaskHandler handles requests to add a new task to the modifier.
func AddModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	var req models.AddModifierTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Basic validation (can be expanded)
	if req.ParameterizedURLID == 0 && req.HTTPTrafficLogID == 0 {
		http.Error(w, "Either parameterized_url_id or http_traffic_log_id is required", http.StatusBadRequest)
		return
	}

	task, err := database.CreateModifierTaskFromSource(req)
	if err != nil {
		// Check if the underlying error is sql.ErrNoRows, or if our custom "not found" message is present
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
			var sourceDescription string
			if req.HTTPTrafficLogID != 0 {
				sourceDescription = fmt.Sprintf("HTTP traffic log ID %d", req.HTTPTrafficLogID)
			} else if req.ParameterizedURLID != 0 {
				sourceDescription = fmt.Sprintf("Parameterized URL ID %d", req.ParameterizedURLID)
			} else {
				sourceDescription = "source item"
			}
			logger.Info("AddModifierTaskHandler: Source for creating modifier task not found (%s): %v", sourceDescription, err)
			http.Error(w, fmt.Sprintf("Source item (%s) not found.", sourceDescription), http.StatusNotFound)
			return
		}
		logger.Error("AddModifierTaskHandler: Failed to create modifier task: %v", err)
		http.Error(w, "Failed to create modifier task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// GetModifierTasksHandler retrieves all tasks for the modifier.
func GetModifierTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Add query param handling for target_id if needed in the future
	tasks, err := database.GetModifierTasks(0) // 0 for no target_id filter for now
	if err != nil {
		http.Error(w, "Failed to retrieve modifier tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []models.ModifierTask{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetModifierTaskDetailsHandler retrieves details for a specific modifier task.
func GetModifierTaskDetailsHandler(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid task_id in path", http.StatusBadRequest)
		return
	}

	task, err := database.GetModifierTaskByID(taskID)
	if err != nil {
		http.Error(w, "Failed to retrieve modifier task details: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "Modifier task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// DeleteModifierTaskHandler handles requests to delete a specific modifier task.
func DeleteModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "task_id") // Get task_id from URL path
	if taskIDStr == "" {
		http.Error(w, "task_id path parameter is required", http.StatusBadRequest)
		return
	}

	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid task_id in path: "+err.Error(), http.StatusBadRequest)
		return
	}

	err = database.DeleteModifierTask(taskID)
	if err != nil {
		// Consider checking for sql.ErrNoRows if DeleteModifierTask returns it
		// to provide a 404 if the task wasn't found.
		http.Error(w, "Failed to delete modifier task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK) // Or http.StatusNoContent (204)
	json.NewEncoder(w).Encode(map[string]string{"message": "Modifier task deleted successfully"})
	logger.Info("Successfully deleted modifier task with ID: %d", taskID)
}

// parseHeadersString converts a multi-line string of headers into an http.Header object.
func parseHeadersString(headerStr string) (http.Header, error) {
	headers := make(http.Header)
	if strings.TrimSpace(headerStr) == "" {
		return headers, nil
	}
	lines := strings.Split(strings.ReplaceAll(headerStr, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			logger.Warn("parseHeadersString: Malformed header line (skipping): %s", line)
			continue // Skip malformed lines instead of erroring out for more flexibility
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			logger.Warn("parseHeadersString: Empty header key in line (skipping): %s", line)
			continue // Skip lines with empty keys
		}
		headers.Add(key, value) // Use Add to support multiple values for the same header key
	}
	return headers, nil
}

// isSafeURLForModifier checks if the URL is safe for the backend to request,
// primarily to mitigate SSRF risks against the server itself.
func isSafeURLForModifier(rawurl string, allowLoopback bool) (bool, error) {
	parsedURL, err := url.Parse(rawurl)
	if err != nil {
		return false, fmt.Errorf("invalid URL: %w", err)
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		// Allow URLs without hostnames (e.g. relative paths if base URL is handled elsewhere, though less common for modifier)
		// Or treat as an error depending on expected use. For now, let's assume it might be valid in some contexts.
		// If the http.NewRequest fails later, that will catch it.
		return true, nil
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		logger.Warn("isSafeURLForModifier: Could not resolve hostname '%s': %v. Proceeding with caution as the user might be targeting non-public DNS.", hostname, err)
		return true, nil // Allow if DNS resolution fails, as user might intend to hit internal-only hostnames.
	}

	for _, ip := range ips {
		if ip.IsLoopback() && !allowLoopback {
			return false, fmt.Errorf("requests to loopback addresses (e.g., localhost, 127.0.0.1) are disallowed by default for security reasons")
		}
		if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return false, fmt.Errorf("requests to link-local addresses (e.g., 169.254.x.x) are disallowed for security reasons")
		}
	}
	return true, nil
}

// ExecuteModifiedRequestHandler handles executing a modified request.
func ExecuteModifiedRequestHandler(w http.ResponseWriter, r *http.Request) {
	var payload executeModifiedRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error decoding request: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(payload.Method) == "" || strings.TrimSpace(payload.URL) == "" {
		logger.Error("ExecuteModifiedRequestHandler: Method and URL are required.")
		http.Error(w, "Method and URL are required.", http.StatusBadRequest)
		return
	}

	parsedHeaders, err := parseHeadersString(payload.Headers)
	if err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error parsing headers: %v", err)
		http.Error(w, "Error parsing headers: "+err.Error(), http.StatusBadRequest)
		return
	}

	// SSRF Check
	// Assume config.GetAppConfig() exists and returns your application's configuration.
	// You'll need to add ModifierAllowLoopback to your config struct (e.g., in models/config.go and load it in config/config.go)
	allowLoopback := config.AppConfig.Proxy.ModifierAllowLoopback
	safe, errSafe := isSafeURLForModifier(payload.URL, allowLoopback)
	if !safe {
		logger.Error("ExecuteModifiedRequestHandler: Unsafe URL for SSRF: %s. Error: %v", payload.URL, errSafe)
		http.Error(w, "The requested URL is considered unsafe: "+errSafe.Error(), http.StatusBadRequest)
		return
	}

	reqBodyReader := strings.NewReader(payload.Body)
	httpRequest, err := http.NewRequest(strings.ToUpper(payload.Method), payload.URL, reqBodyReader)
	if err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error creating new HTTP request: %v", err)
		http.Error(w, "Error creating request: "+err.Error(), http.StatusInternalServerError)
		return
	}
	httpRequest.Header = parsedHeaders

	// Before executing, update the ModifierTask's base request details in the DB
	// with the current values from the payload (which are from the UI).
	if payload.TaskID != nil && *payload.TaskID != 0 {
		reqHeadersForDBJSON, errMarshal := json.Marshal(parsedHeaders) // Marshal http.Header to JSON string
		if errMarshal != nil {
			logger.Error("ExecuteModifiedRequestHandler: Error marshalling request headers for DB update: %v", errMarshal)
			// Decide if this is a fatal error for the operation or just log and continue
		}
		reqBodyForDBBase64 := base64.StdEncoding.EncodeToString([]byte(payload.Body))

		if err := database.UpdateModifierTaskBaseRequestDetails(*payload.TaskID, strings.ToUpper(payload.Method), payload.URL, string(reqHeadersForDBJSON), reqBodyForDBBase64); err != nil {
			logger.Error("ExecuteModifiedRequestHandler: Failed to update modifier task base request details for TaskID %d: %v", *payload.TaskID, err)
			// Decide if this is fatal or log and continue. For now, log and continue.
		}
	}

	// Configure an HTTP client (e.g., to skip TLS verification for testing)
	// Use a configuration setting for InsecureSkipVerify.
	// Assume config.GetAppConfig() exists and returns your application's configuration.
	// You'll need to add ModifierSkipTLSVerify to your config struct.
	skipTLSVerify := config.AppConfig.Proxy.ModifierSkipTLSVerify
	if skipTLSVerify {
		logger.Warn("ExecuteModifiedRequestHandler: TLS certificate verification is DISABLED for outgoing modified requests.")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second, // Set a reasonable timeout
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Do not follow redirects automatically
		},
	}

	startTime := time.Now()
	httpResponse, err := client.Do(httpRequest)
	durationMs := time.Since(startTime).Milliseconds()

	apiResponse := executeModifiedResponsePayload{DurationMs: durationMs}

	if err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error executing request to %s: %v", payload.URL, err)
		apiResponse.Error = "Failed to execute request: " + err.Error()
		// Still try to send what we have, maybe a partial response or just the error
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) // Or a specific error code if preferred, but frontend expects JSON
		json.NewEncoder(w).Encode(apiResponse)
		return
	}
	defer httpResponse.Body.Close()

	// Always set these basic response details, even if body reading fails later
	apiResponse.StatusCode = httpResponse.StatusCode
	apiResponse.StatusText = httpResponse.Status
	apiResponse.Headers = httpResponse.Header // Start with original headers

	var responseBodyBytes []byte
	var readErr error
	removedContentEncodingHeaders := false

	// Handle content encoding (decompression)
	contentEncoding := httpResponse.Header.Get("Content-Encoding")
	logger.Info("ExecuteModifiedRequestHandler: Response Content-Encoding from server: %s", contentEncoding)

	switch strings.ToLower(contentEncoding) {
	case "br":
		logger.Info("ExecuteModifiedRequestHandler: Decompressing Brotli body.")
		brReader := brotli.NewReader(httpResponse.Body)
		responseBodyBytes, readErr = io.ReadAll(brReader)
		removedContentEncodingHeaders = true
	case "gzip":
		logger.Info("ExecuteModifiedRequestHandler: Decompressing Gzip body.")
		gzReader, gzErr := gzip.NewReader(httpResponse.Body)
		if gzErr != nil { // If gzip.NewReader itself fails
			logger.Error("ExecuteModifiedRequestHandler: Failed to create gzip reader: %v. This indicates a problem with the gzip stream.", gzErr)
			readErr = gzErr // Propagate the error from gzip.NewReader
			// responseBodyBytes will remain empty or nil, which is fine as readErr is set
		} else {
			responseBodyBytes, readErr = io.ReadAll(gzReader)
			gzReader.Close()    // Important to close the gzip reader
			if readErr == nil { // Only set if decompression and read were successful
				removedContentEncodingHeaders = true
			}
		}
	default:
		logger.Info("ExecuteModifiedRequestHandler: No/Unknown Content-Encoding ('%s'), reading body as is.", contentEncoding)
		responseBodyBytes, readErr = io.ReadAll(httpResponse.Body)
	}

	if readErr != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error reading/decompressing response body: %v", readErr)
		// If an error occurred during body read/decompression, set an error message
		// and ensure the body is empty or indicates an error, rather than sending potentially corrupt data.
		apiResponse.Error = "Error reading or decompressing response body: " + readErr.Error()
		apiResponse.Body = "" // Send an empty body if read failed
		// Populate other fields even if body reading failed
		apiResponse.StatusCode = httpResponse.StatusCode
		apiResponse.StatusText = httpResponse.Status
		apiResponse.Headers = httpResponse.Header
	} else {
		apiResponse.Body = base64.StdEncoding.EncodeToString(responseBodyBytes)
	}

	if removedContentEncodingHeaders {
		// If we decompressed, the Content-Encoding and Content-Length headers are no longer valid for the data we're sending.
		newHeaders := make(http.Header)
		for k, vv := range httpResponse.Header {
			lowerK := strings.ToLower(k)
			if lowerK != "content-encoding" && lowerK != "content-length" {
				newHeaders[k] = vv
			}
		}
		apiResponse.Headers = newHeaders
	}

	// Log the executed request and response
	var targetIDForLog *int64
	if payload.TaskID != nil {
		taskDetails, taskErr := database.GetModifierTaskByID(*payload.TaskID)
		if taskErr == nil && taskDetails != nil && taskDetails.TargetID.Valid {
			targetIDForLog = &taskDetails.TargetID.Int64
		} else if taskErr != nil {
			logger.Error("ExecuteModifiedRequestHandler: Failed to get task details for TargetID: %v", taskErr)
		}
	}

	reqHeadersForLogJSON, _ := json.Marshal(parsedHeaders)        // parsedHeaders is http.Header
	respHeadersForLogJSON, _ := json.Marshal(apiResponse.Headers) // apiResponse.Headers is map[string][]string

	logEntry := &models.HTTPTrafficLog{
		TargetID:             targetIDForLog,
		Timestamp:            startTime,
		RequestMethod:        models.NullString(strings.ToUpper(payload.Method)),
		RequestURL:           models.NullString(payload.URL),       // This was already correct
		RequestHTTPVersion:   models.NullString(httpRequest.Proto), // Corrected
		RequestHeaders:       models.NullString(string(reqHeadersForLogJSON)),
		RequestBody:          []byte(payload.Body),
		ResponseStatusCode:   apiResponse.StatusCode,
		ResponseReasonPhrase: models.NullString(strings.TrimPrefix(apiResponse.StatusText, fmt.Sprintf("%d ", apiResponse.StatusCode))), // Corrected
		ResponseHTTPVersion:  models.NullString(httpResponse.Proto),                                                                     // Corrected
		ResponseHeaders:      models.NullString(string(respHeadersForLogJSON)),                                                          // Corrected
		ResponseBody:         responseBodyBytes,                                                                                         // Decompressed body
		ResponseContentType:  models.NullString(httpResponse.Header.Get("Content-Type")),                                                // Corrected
		ResponseBodySize:     int64(len(responseBodyBytes)),
		DurationMs:           durationMs,
		IsHTTPS:              strings.HasPrefix(strings.ToLower(payload.URL), "https://"),
		IsPageCandidate:      strings.Contains(strings.ToLower(httpResponse.Header.Get("Content-Type")), "text/html"),
		LogSource:            models.NullString("Modifier"),         // Set log source
		PageSitemapID:        sql.NullInt64{Valid: false},           // No direct page sitemap association here
		SourceModifierTaskID: sql.NullInt64{Int64: 0, Valid: false}, // Default to invalid
	}
	if payload.TaskID != nil {
		logEntry.SourceModifierTaskID = sql.NullInt64{Int64: *payload.TaskID, Valid: true}
	}

	if logID, dbLogErr := database.LogExecutedModifierRequest(logEntry); dbLogErr != nil {
		logger.Error("ExecuteModifiedRequestHandler: Failed to log executed modified request: %v", dbLogErr)
		// Continue to send response to client even if logging fails
	} else {
		// Successfully logged, now update the modifier task with the new log ID
		if payload.TaskID != nil {
			if updateErr := database.UpdateModifierTaskLastExecutedLogID(*payload.TaskID, logID); updateErr != nil {
				logger.Error("ExecuteModifiedRequestHandler: Failed to update modifier task %d with last_executed_log_id %d: %v", *payload.TaskID, logID, updateErr)
			}
		}
	}

	// After all processing, before sending the response:
	logger.Debug("ExecuteModifiedRequestHandler: Final apiResponse being sent to frontend: StatusCode=%d, StatusText='%s', HeadersCount=%d, BodyLength(Base64)=%d, Error='%s'",
		apiResponse.StatusCode, apiResponse.StatusText, len(apiResponse.Headers), len(apiResponse.Body), apiResponse.Error)
	if len(apiResponse.Headers) == 0 && apiResponse.StatusCode != 0 && apiResponse.Error == "" { // If status is set, no error, but headers are empty
		logger.Warn("ExecuteModifiedRequestHandler: apiResponse.Headers is empty but StatusCode is %d and no error reported. Original httpResponse.Header count: %d", apiResponse.StatusCode, len(httpResponse.Header))
		// For deeper debugging, you can marshal and log the actual headers:
		// originalHeadersJSON, _ := json.Marshal(httpResponse.Header)
		// logger.Debug("ExecuteModifiedRequestHandler: Original httpResponse.Header: %s", string(originalHeadersJSON))
		// apiResponseHeadersJSON, _ := json.Marshal(apiResponse.Headers)
		// logger.Debug("ExecuteModifiedRequestHandler: Final apiResponse.Headers: %s", string(apiResponseHeadersJSON))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiResponse)
	logger.Info("ExecuteModifiedRequestHandler: Executed %s %s, Status: %d", payload.Method, payload.URL, apiResponse.StatusCode)
}

// UpdateModifierTaskHandler handles updating parts of a task, like its name.
func UpdateModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid task_id in path", http.StatusBadRequest)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "Task name cannot be empty", http.StatusBadRequest)
		return
	}

	updatedTask, err := database.UpdateModifierTaskName(taskID, req.Name)
	if err != nil {
		// Check if the error is due to task not found
		// This depends on how UpdateModifierTaskName or GetModifierTaskByID (if called within) handles it.
		// For now, assuming a general server error.
		logger.Error("UpdateModifierTaskHandler: Error updating task name for ID %d: %v", taskID, err)
		http.Error(w, "Failed to update modifier task name: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if updatedTask == nil { // Should be handled by error above if GetModifierTaskByID returns sql.ErrNoRows
		http.Error(w, "Modifier task not found after update attempt", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedTask)
	logger.Info("Successfully updated name for modifier task ID %d to '%s'", taskID, req.Name)
}

// CloneModifierTaskHandler handles cloning an existing modifier task.
func CloneModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	originalTaskIDStr := chi.URLParam(r, "task_id")
	originalTaskID, err := strconv.ParseInt(originalTaskIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid original task_id in path", http.StatusBadRequest)
		return
	}

	originalTask, err := database.GetModifierTaskByID(originalTaskID)
	if err != nil {
		logger.Error("CloneModifierTaskHandler: Error fetching original task ID %d: %v", originalTaskID, err)
		http.Error(w, "Failed to retrieve original task for cloning", http.StatusInternalServerError)
		return
	}
	if originalTask == nil {
		http.Error(w, "Original modifier task not found for cloning", http.StatusNotFound)
		return
	}

	clonedTask, err := database.CloneModifierTaskDB(originalTaskID)
	if err != nil {
		logger.Error("CloneModifierTaskHandler: Error cloning task ID %d: %v", originalTaskID, err)
		http.Error(w, "Failed to clone modifier task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(clonedTask)
	logger.Info("Successfully cloned modifier task ID %d to new task ID %d", originalTaskID, clonedTask.ID)
}

// UpdateModifierTasksOrderHandler handles updating the display order of modifier tasks.
func UpdateModifierTasksOrderHandler(w http.ResponseWriter, r *http.Request) {
	var taskOrders map[string]int // Expecting {"taskID_string": order_int}
	if err := json.NewDecoder(r.Body).Decode(&taskOrders); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	orders := make(map[int64]int)
	for taskIDStr, order := range taskOrders {
		taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
		if err != nil {
			logger.Error("UpdateModifierTasksOrderHandler: Invalid task_id '%s' in payload: %v", taskIDStr, err)
			http.Error(w, "Invalid task_id in payload: "+taskIDStr, http.StatusBadRequest)
			return
		}
		orders[taskID] = order
	}

	if err := database.UpdateModifierTasksOrder(orders); err != nil {
		logger.Error("UpdateModifierTasksOrderHandler: Error updating task order in DB: %v", err)
		http.Error(w, "Failed to update task order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Task order updated successfully."})
	logger.Info("Successfully updated display order for %d modifier tasks.", len(orders))
}

// DeleteAllModifierTasksForTargetHandler handles requests to delete all modifier tasks for a specific target.
func DeleteAllModifierTasksForTargetHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := chi.URLParam(r, "target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		logger.Error("DeleteAllModifierTasksForTargetHandler: Invalid target_id in path: %v", err)
		http.Error(w, "Invalid target_id in path", http.StatusBadRequest)
		return
	}

	if targetID == 0 {
		logger.Error("DeleteAllModifierTasksForTargetHandler: target_id cannot be zero.")
		http.Error(w, "target_id cannot be zero", http.StatusBadRequest)
		return
	}

	rowsAffected, err := database.DeleteModifierTasksByTargetID(targetID)
	if err != nil {
		logger.Error("DeleteAllModifierTasksForTargetHandler: Error deleting tasks for target %d: %v", targetID, err)
		http.Error(w, "Failed to delete modifier tasks for target", http.StatusInternalServerError)
		return
	}

	logger.Info("Successfully deleted %d modifier tasks for target ID %d", rowsAffected, targetID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       fmt.Sprintf("Successfully deleted %d modifier tasks for target ID %d.", rowsAffected, targetID),
		"rows_affected": rowsAffected,
	})
}
