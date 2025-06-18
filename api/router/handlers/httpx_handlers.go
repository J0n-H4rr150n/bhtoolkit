package handlers

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// HttpxResult defines the structure for parsing httpx JSON output.
// Add fields here as needed based on the httpx flags used.
type HttpxResult struct {
	Timestamp     string   `json:"timestamp"`
	Input         string   `json:"input"` // The input domain name
	URL           string   `json:"url"`
	StatusCode    int      `json:"status_code"`
	ContentLength int64    `json:"content_length"`
	Title         string   `json:"title"`
	WebServer     string   `json:"webserver"`
	Technologies  []string `json:"tech"`
	FullJson      string   `json:"-"` // To store the raw JSON line
}

// HttpxTaskStatus defines the structure for tracking httpx scan progress.
type HttpxTaskStatus struct {
	IsRunning        bool      `json:"is_running"`
	Message          string    `json:"message"`
	DomainsTotal     int       `json:"domains_total"`
	DomainsProcessed int       `json:"domains_processed"`
	LastError        string    `json:"last_error,omitempty"`
	StartTime        time.Time `json:"start_time"`
}

var (
	httpxTaskStatuses   = make(map[int64]*HttpxTaskStatus)
	httpxTaskStatusLock = &sync.RWMutex{}
)

// isBetterResult determines if newResult is "better" than oldResult.
// Criteria: HTTPS > HTTP, 2xx > other statuses, has title > no title.
func isBetterResult(newResult, oldResult HttpxResult) bool {
	// Prefer HTTPS
	newIsHTTPS := strings.HasPrefix(strings.ToLower(newResult.URL), "https://")
	oldIsHTTPS := strings.HasPrefix(strings.ToLower(oldResult.URL), "https://")
	if newIsHTTPS && !oldIsHTTPS {
		return true
	}
	if !newIsHTTPS && oldIsHTTPS {
		return false
	}

	// Prefer 2xx status codes
	newIs2xx := newResult.StatusCode >= 200 && newResult.StatusCode < 300
	oldIs2xx := oldResult.StatusCode >= 200 && oldResult.StatusCode < 300
	if newIs2xx && !oldIs2xx {
		return true
	}
	if !newIs2xx && oldIs2xx {
		return false
	}

	// If both are 2xx or both are not 2xx, prefer one with a title
	newHasTitle := strings.TrimSpace(newResult.Title) != ""
	oldHasTitle := strings.TrimSpace(oldResult.Title) != ""
	if newHasTitle && !oldHasTitle {
		return true
	}
	if !newHasTitle && oldHasTitle {
		return false
	}

	// If status code significance is similar (e.g. both 2xx or both non-2xx)
	// and title presence is similar.
	// Prefer non-zero status code if the other is zero (less likely from httpx but a safeguard)
	if newResult.StatusCode != 0 && oldResult.StatusCode == 0 {
		return true
	}
	if newResult.StatusCode == 0 && oldResult.StatusCode != 0 {
		return false
	}
	
	// If all primary criteria are equal, we might prefer the one with more technologies detected
	if len(newResult.Technologies) > len(oldResult.Technologies) {
		return true
	}
	if len(newResult.Technologies) < len(oldResult.Technologies) {
		return false
	}

	return false // Default to not replacing if not clearly better
}

// RunHttpxScan executes httpx against the provided domains and logs the output.
func RunHttpxScan(targetID int64, domains []models.Domain) {
	if len(domains) == 0 {
		logger.Info("RunHttpxScan: No domains provided for target ID %d. Skipping scan.", targetID)
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.IsRunning = false
			status.Message = "No domains provided to scan."
		} else {
			httpxTaskStatuses[targetID] = &HttpxTaskStatus{
				IsRunning:        false,
				Message:          "No domains provided to scan.",
				DomainsTotal:     0,
				DomainsProcessed: 0,
				StartTime:        time.Now(),
			}
		}
		httpxTaskStatusLock.Unlock()
		return
	}

	httpxTaskStatusLock.Lock()
	httpxTaskStatuses[targetID] = &HttpxTaskStatus{
		IsRunning:        true,
		Message:          "Initializing httpx scan...",
		DomainsTotal:     len(domains),
		DomainsProcessed: 0,
		StartTime:        time.Now(),
	}
	httpxTaskStatusLock.Unlock()

	defer func() {
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.IsRunning = false
			// Check if the message indicates it was still in an early phase
			if status.Message == "Initializing httpx scan..." ||
				strings.HasPrefix(status.Message, "Starting httpx scan...") ||
				(strings.HasPrefix(status.Message, "Processing...") && status.DomainsProcessed < status.DomainsTotal) {
				if status.LastError != "" {
					status.Message = "Httpx scan failed: " + status.LastError
				} else if status.DomainsProcessed < status.DomainsTotal {
					status.Message = fmt.Sprintf("Httpx scan interrupted. Processed %d of %d domains.", status.DomainsProcessed, status.DomainsTotal)
				} else { // Should ideally be covered by the final success message
					status.Message = fmt.Sprintf("Httpx scan finished. Processed %d of %d domains.", status.DomainsProcessed, status.DomainsTotal)
				}
			}
			// If LastError is set and message doesn't reflect failure, append/update.
			// This part might need refinement based on how messages are set throughout.
			if status.LastError != "" && !strings.Contains(status.Message, "failed") && !strings.Contains(status.Message, "error") && !strings.Contains(status.Message, "timed out") {
                 status.Message = fmt.Sprintf("%s Last error: %s", status.Message, status.LastError)
            }
		}
		httpxTaskStatusLock.Unlock()
	}()

	_, err := exec.LookPath("httpx")
	if err != nil {
		errMsg := fmt.Sprintf("httpx command not found in PATH: %v", err)
		logger.Error("RunHttpxScan: %s for target ID %d", errMsg, targetID)
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.Message = "httpx command not found."
			status.LastError = errMsg
		}
		httpxTaskStatusLock.Unlock()
		return
	}

	logger.Info("RunHttpxScan: Starting httpx scan for target ID %d on %d domains.", targetID, len(domains))
	httpxTaskStatusLock.Lock()
	if status, ok := httpxTaskStatuses[targetID]; ok {
		status.Message = fmt.Sprintf("Starting httpx scan for %d domains...", len(domains))
	}
	httpxTaskStatusLock.Unlock()

	var domainListBuilder strings.Builder
	domainMap := make(map[string]int64) // Map input domain name to its DB ID
	for _, d := range domains {
		domainListBuilder.WriteString(d.DomainName + "\n")
		domainMap[d.DomainName] = d.ID
	}
	domainInput := domainListBuilder.String()

	/*
	argsFull := []string{
		"-json", "-status-code", "-content-length", "-title", "-tech-detect", "-server",
		"-cdn", "-websocket", "-vhost", "-pipeline", "-http2", "-follow-redirects",
		"-tls-grab", "-csp-probe", "-tls-probe", "-jarm", "-asn", "-favicon",
		"-silent", "-no-color", "-timeout", "10", "-threads", "25", "-retries", "1",
		"-random-agent", "-probe", "-ports", "80,443,8000,8008,8080,8443,3000,5000",
	}*/

	args := []string{
		"-json", "-status-code", "-content-length", "-title", "-tech-detect", "-server",
		"-cdn", "-websocket", "-vhost", "-pipeline", "-http2", "-follow-redirects",
		"-silent", "-no-color", "-timeout", "10", "-threads", "25", "-retries", "1",
		"-random-agent",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "httpx", args...)
	cmd.Stdin = strings.NewReader(domainInput)

	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	err = cmd.Run()

	if errBuffer.Len() > 0 {
		stderrContent := errBuffer.String()
		logger.Warn("RunHttpxScan: httpx stderr for target ID %d: %s", targetID, stderrContent)
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			if status.LastError == "" {
				status.LastError = "httpx stderr: " + strings.Split(stderrContent, "\n")[0] // Store first line of stderr
			}
		}
		httpxTaskStatusLock.Unlock()
	}

	if ctx.Err() == context.DeadlineExceeded {
		errMsg := "Command execution exceeded 30 minutes."
		logger.Error("RunHttpxScan: httpx command timed out for target ID %d.", targetID)
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.Message = "Httpx scan timed out."
			status.LastError = errMsg
		}
		httpxTaskStatusLock.Unlock()
		return
	}
	if err != nil {
		errMsg := fmt.Sprintf("httpx execution error: %v", err)
		logger.Error("RunHttpxScan: %s for target ID %d.", errMsg, targetID)
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.Message = "Httpx scan execution failed."
			if status.LastError == "" { // Don't overwrite a more specific stderr message
				status.LastError = errMsg
			}
		}
		httpxTaskStatusLock.Unlock()
		return
	}

	// --- Process results ---
	resultsByInput := make(map[string][]HttpxResult) // Group results by input domain
	scanner := bufio.NewScanner(&outBuffer)
	for scanner.Scan() {
		line := scanner.Text()
		var result HttpxResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			logger.Error("RunHttpxScan: Error unmarshalling httpx JSON line for target ID %d: %v. Line: %s", targetID, err, line)
			continue
		}
		result.FullJson = line // Store the raw JSON line
		resultsByInput[result.Input] = append(resultsByInput[result.Input], result)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		scanErrMsg := fmt.Sprintf("Error reading httpx output: %v", scanErr)
		logger.Error("RunHttpxScan: %s for target ID %d", scanErrMsg, targetID)
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.Message = "Error reading httpx output."
			status.LastError = scanErrMsg
		}
		httpxTaskStatusLock.Unlock()
		// Continue to process any results gathered so far, even if scanner stopped with an error
	}

	processedDomainInputsCount := 0
	for inputDomain, domainResults := range resultsByInput {
		domainID, ok := domainMap[inputDomain]
		if !ok {
			logger.Warn("RunHttpxScan: Input domain '%s' from httpx output not found in initial domain map for target %d. Skipping.", inputDomain, targetID)
			continue
		}

		if len(domainResults) == 0 {
			// This case should not happen if inputDomain is a key in resultsByInput,
			// but good for safety.
			processedDomainInputsCount++ // Count as processed even if no valid results to update DB
			httpxTaskStatusLock.Lock()
			if status, ok := httpxTaskStatuses[targetID]; ok {
				status.DomainsProcessed = processedDomainInputsCount
				status.Message = fmt.Sprintf("Processing... %d of %d domains processed.", status.DomainsProcessed, status.DomainsTotal)
			}
			httpxTaskStatusLock.Unlock()
			continue
		}

		// Select the best result for this input domain
		bestResult := domainResults[0]
		for i := 1; i < len(domainResults); i++ {
			if isBetterResult(domainResults[i], bestResult) {
				bestResult = domainResults[i]
			}
		}

		// Construct update data from the best result
		domainUpdateData := models.Domain{
			ID:                domainID,
			HTTPStatusCode:    sql.NullInt64{Int64: int64(bestResult.StatusCode), Valid: bestResult.StatusCode != 0},
			HTTPContentLength: sql.NullInt64{Int64: bestResult.ContentLength, Valid: true},
			HTTPTitle:         models.NullString(bestResult.Title),
			HTTPServer:        models.NullString(bestResult.WebServer),
			HTTPTech:          models.NullString(strings.Join(bestResult.Technologies, ",")),
			HttpxFullJson:     models.NullString(bestResult.FullJson),
		}

		errDbUpdate := database.UpdateDomainWithHttpxResult(domainUpdateData)
		if errDbUpdate != nil {
			dbErrMsg := fmt.Sprintf("Failed to update domain ID %d (input: %s) with best httpx result: %v", domainID, inputDomain, errDbUpdate)
			logger.Error("RunHttpxScan: %s", dbErrMsg)
			httpxTaskStatusLock.Lock()
			if status, ok := httpxTaskStatuses[targetID]; ok {
				status.LastError = dbErrMsg // Store last DB error
			}
			httpxTaskStatusLock.Unlock()
		} else {
			logger.Info("RunHttpxScan: Successfully updated domain ID %d (input: %s) with best httpx result.", domainID, inputDomain)
		}

		processedDomainInputsCount++
		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.DomainsProcessed = processedDomainInputsCount
			status.Message = fmt.Sprintf("Processing... %d of %d domains processed.", status.DomainsProcessed, status.DomainsTotal)
		}
		httpxTaskStatusLock.Unlock()
	}

	// After processing domains with results, identify those that were scanned but yielded no output from httpx
	// These are domains in the initial `domains` slice that are not keys in `resultsByInput`.
	initialDomainNames := make(map[string]models.Domain)
	for _, d := range domains {
		initialDomainNames[d.DomainName] = d
	}

	for inputName, domainDetail := range initialDomainNames {
		if _, found := resultsByInput[inputName]; !found {
			// This domain was in the input list to httpx but httpx produced no output for it.
			// We'll mark it as scanned by setting a minimal httpx_full_json and updating its timestamp.
			logger.Info("RunHttpxScan: Domain '%s' (ID: %d) yielded no direct results from httpx. Marking as scanned.", domainDetail.DomainName, domainDetail.ID)

			noResultJson := `{"status_code": null, "message": "No HTTP response on probed ports"}` // Example "no result" marker
			domainUpdateData := models.Domain{
				ID:                domainDetail.ID,
				HTTPStatusCode:    sql.NullInt64{Valid: false}, // No specific status code
				HTTPContentLength: sql.NullInt64{Valid: false}, // No content length
				HTTPTitle:         models.NullString(""),
				HTTPServer:        models.NullString(""),
				HTTPTech:          models.NullString(""),
				HttpxFullJson:     models.NullString(noResultJson), // Store the marker
			}

			errDbUpdate := database.UpdateDomainWithHttpxResult(domainUpdateData)
			if errDbUpdate != nil {
				dbErrMsg := fmt.Sprintf("Failed to mark domain ID %d (input: %s) as scanned (no direct results): %v", domainDetail.ID, domainDetail.DomainName, errDbUpdate)
				logger.Error("RunHttpxScan: %s", dbErrMsg)
				// Optionally update task status with this error, though it's a per-domain update issue
			} else {
				logger.Info("RunHttpxScan: Successfully marked domain ID %d (input: %s) as scanned (no direct results).", domainDetail.ID, domainDetail.DomainName)
			}
		}
	}

	httpxTaskStatusLock.Lock()
	var finalProcessedCount, finalTotalCount int
	if status, ok := httpxTaskStatuses[targetID]; ok {
		finalProcessedCount = status.DomainsProcessed
		finalTotalCount = status.DomainsTotal
		finalMessage := fmt.Sprintf("Httpx scan completed. Scanned %d domains; %d yielded results.", finalTotalCount, finalProcessedCount)
		if status.LastError != "" {
			finalMessage = fmt.Sprintf("Httpx scan completed with errors. Scanned %d domains; %d yielded results. Last error: %s", finalTotalCount, finalProcessedCount, status.LastError)
		}
		status.Message = finalMessage
		// IsRunning will be set to false by the defer function
	}
	httpxTaskStatusLock.Unlock()
	logger.Info("RunHttpxScan: Finished httpx scan for target ID %d. Scanned %d domains; %d yielded results.", targetID, finalTotalCount, finalProcessedCount)
}

// GetHttpxStatusHandler returns the current status of httpx scans for a target.
// @Summary Get httpx Task Status
// @Description Retrieves the current status of background httpx tasks for a specific target.
// @Tags Domains
// @Produce json
// @Param target_id query int true "ID of the target"
// @Success 200 {object} HttpxTaskStatus
// @Failure 400 {object} models.ErrorResponse "Invalid or missing target_id"
// @Router /httpx/status [get]
func GetHttpxStatusHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id", http.StatusBadRequest)
		return
	}

	httpxTaskStatusLock.RLock()
	status, ok := httpxTaskStatuses[targetID]
	httpxTaskStatusLock.RUnlock()

	if !ok {
		status = &HttpxTaskStatus{
			IsRunning: false,
			Message:   "No active httpx scan for this target or status not initialized.",
			StartTime: time.Now(), // Or a zero time if preferred
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
