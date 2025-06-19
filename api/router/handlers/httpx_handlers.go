package handlers

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"toolkit/config" // Added for accessing proxy port
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// HttpxResult defines the structure for parsing httpx JSON output.
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
	DomainsProcessed int       `json:"domains_processed"`  // Domains attempted by httpx
	DomainsUpdatedDB int       `json:"domains_updated_db"` // Domains for which DB update was attempted
	LastError        string    `json:"last_error,omitempty"`
	StartTime        time.Time `json:"start_time"`
}

var (
	httpxTaskStatuses        = make(map[int64]*HttpxTaskStatus)
	httpxTaskStatusLock      = &sync.RWMutex{}
	httpxScanCancelFuncs     = make(map[int64]context.CancelFunc) // New map for cancel functions
	httpxScanCancelFuncsLock = &sync.Mutex{}                      // Mutex for the cancel functions map
)

// isBetterResult determines if newResult is "better" than oldResult.
func isBetterResult(newResult, oldResult HttpxResult) bool {
	newIsHTTPS := strings.HasPrefix(strings.ToLower(newResult.URL), "https://")
	oldIsHTTPS := strings.HasPrefix(strings.ToLower(oldResult.URL), "https://")
	if newIsHTTPS && !oldIsHTTPS {
		return true
	}
	if !newIsHTTPS && oldIsHTTPS {
		return false
	}
	newIs2xx := newResult.StatusCode >= 200 && newResult.StatusCode < 300
	oldIs2xx := oldResult.StatusCode >= 200 && oldResult.StatusCode < 300
	if newIs2xx && !oldIs2xx {
		return true
	}
	if !newIs2xx && oldIs2xx {
		return false
	}
	newHasTitle := strings.TrimSpace(newResult.Title) != ""
	oldHasTitle := strings.TrimSpace(oldResult.Title) != ""
	if newHasTitle && !oldHasTitle {
		return true
	}
	if !newHasTitle && oldHasTitle {
		return false
	}
	if newResult.StatusCode != 0 && oldResult.StatusCode == 0 {
		return true
	}
	if newResult.StatusCode == 0 && oldResult.StatusCode != 0 {
		return false
	}
	if len(newResult.Technologies) > len(oldResult.Technologies) {
		return true
	}
	if len(newResult.Technologies) < len(oldResult.Technologies) {
		return false
	}
	return false
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
				DomainsUpdatedDB: 0,
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
		DomainsUpdatedDB: 0,
		StartTime:        time.Now(),
	}
	httpxTaskStatusLock.Unlock()

	// Overall context for the entire scan operation, including cancellation capability
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)

	// Store the cancel function
	httpxScanCancelFuncsLock.Lock()
	httpxScanCancelFuncs[targetID] = cancel
	httpxScanCancelFuncsLock.Unlock()

	defer func() {
		// Ensure context is cancelled when RunHttpxScan exits, regardless of reason
		cancel()

		// Remove cancel func from the map
		httpxScanCancelFuncsLock.Lock()
		delete(httpxScanCancelFuncs, targetID)
		httpxScanCancelFuncsLock.Unlock()

		// Set final status
		httpxTaskStatusLock.Lock()
		defer httpxTaskStatusLock.Unlock()

		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.IsRunning = false
			// Check context error first for cancellation
			if errors.Is(ctx.Err(), context.Canceled) {
				status.Message = "Httpx scan cancelled by user."
				if status.LastError == "" {
					status.LastError = "Operation cancelled."
				}
			} else if status.LastError != "" {
				status.Message = fmt.Sprintf("Httpx scan finished with errors: %s. Processed %d/%d domains. DB updates for %d domains.", status.LastError, status.DomainsProcessed, status.DomainsTotal, status.DomainsUpdatedDB)
			} else if status.DomainsProcessed < status.DomainsTotal && status.Message != "Httpx scan cancelled by user." {
				status.Message = fmt.Sprintf("Httpx scan interrupted. Processed %d/%d domains. DB updates for %d domains.", status.DomainsProcessed, status.DomainsTotal, status.DomainsUpdatedDB)
			} else if status.Message != "Httpx scan cancelled by user." {
				status.Message = fmt.Sprintf("Httpx scan completed. Processed %d/%d domains. DB updates for %d domains.", status.DomainsProcessed, status.DomainsTotal, status.DomainsUpdatedDB)
			}
		}
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

	proxyPort := config.AppConfig.Proxy.Port
	proxyURL := ""
	if proxyPort != "" {
		proxyURL = fmt.Sprintf("http://127.0.0.1:%s", proxyPort)
	}

	baseArgs := []string{
		"-json", "-status-code", "-content-length", "-title", "-tech-detect", "-server",
		"-cdn", "-websocket", "-vhost", "-pipeline", "-http2", "-follow-redirects",
		"-silent", "-no-color", "-timeout", "10", "-threads", "25", "-retries", "1",
		"-random-agent",
	}
	if proxyURL != "" {
		baseArgs = append(baseArgs, "-proxy", proxyURL)
		logger.Info("RunHttpxScan: Using proxy %s for httpx scan.", proxyURL)
	}

	const batchSize = 50
	numDomains := len(domains)
	var cumulativeDomainsUpdatedDB int64 = 0

	for i := 0; i < numDomains; i += batchSize {
		// Check for context cancellation before starting a new batch
		select {
		case <-ctx.Done():
			logger.Info("RunHttpxScan: Context cancelled before processing batch %d for target ID %d. Error: %v", (i/batchSize)+1, targetID, ctx.Err())
			// The defer function will handle setting the final status message.
			return // Exit the scan early
		default:
			// Continue processing
		}

		end := i + batchSize
		if end > numDomains {
			end = numDomains
		}
		currentBatchDomains := domains[i:end]
		batchDomainsProcessedThisIteration := 0

		var batchDomainListBuilder strings.Builder
		for _, d := range currentBatchDomains {
			batchDomainListBuilder.WriteString(d.DomainName + "\n")
		}
		batchDomainInput := batchDomainListBuilder.String()

		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.Message = fmt.Sprintf("Processing batch %d of %d (%d domains)...", (i/batchSize)+1, (numDomains+batchSize-1)/batchSize, len(currentBatchDomains))
		}
		httpxTaskStatusLock.Unlock()

		var batchOutBuffer, batchErrBuffer bytes.Buffer
		batchCmd := exec.CommandContext(ctx, "httpx", baseArgs...)
		batchCmd.Stdin = strings.NewReader(batchDomainInput)
		batchCmd.Stdout = &batchOutBuffer
		batchCmd.Stderr = &batchErrBuffer

		batchCmdErr := batchCmd.Run()
		batchDomainsProcessedThisIteration = len(currentBatchDomains)

		batchResultsMap := make(map[string][]HttpxResult)

		if batchErrBuffer.Len() > 0 {
			stderrContent := batchErrBuffer.String()
			logger.Warn("RunHttpxScan: httpx stderr for batch %d, target ID %d: %s", (i/batchSize)+1, targetID, stderrContent)
		}

		if batchCmdErr != nil {
			// Check if the error is due to context cancellation
			if errors.Is(batchCmdErr, context.Canceled) || errors.Is(batchCmdErr, context.DeadlineExceeded) {
				logger.Info("RunHttpxScan: httpx command for batch %d cancelled or timed out for target ID %d. Error: %v", (i/batchSize)+1, targetID, batchCmdErr)
				// The main context check at the loop start or the defer will handle the overall status.
				// We still update processed count for this batch as an attempt was made.
			} else {
				batchErrMsg := fmt.Sprintf("httpx execution error for batch %d: %v", (i/batchSize)+1, batchCmdErr)
				logger.Error("RunHttpxScan: %s for target ID %d.", batchErrMsg, targetID)
				httpxTaskStatusLock.Lock()
				if status, ok := httpxTaskStatuses[targetID]; ok {
					if status.LastError == "" {
						status.LastError = batchErrMsg
					}
				}
				httpxTaskStatusLock.Unlock()
			}
		} else {
			batchScanner := bufio.NewScanner(&batchOutBuffer)
			for batchScanner.Scan() {
				line := batchScanner.Text()
				var result HttpxResult
				if err := json.Unmarshal([]byte(line), &result); err != nil {
					logger.Error("RunHttpxScan: Error unmarshalling httpx JSON line for batch, target ID %d: %v. Line: %s", targetID, err, line)
					continue
				}
				result.FullJson = line
				batchResultsMap[result.Input] = append(batchResultsMap[result.Input], result)
			}
			if scanErr := batchScanner.Err(); scanErr != nil {
				scanErrMsg := fmt.Sprintf("Error reading httpx output for batch %d: %v", (i/batchSize)+1, scanErr)
				logger.Error("RunHttpxScan: %s for target ID %d", scanErrMsg, targetID)
				httpxTaskStatusLock.Lock()
				if status, ok := httpxTaskStatuses[targetID]; ok {
					if status.LastError == "" {
						status.LastError = scanErrMsg
					}
				}
				httpxTaskStatusLock.Unlock()
			}
		}

		var batchDomainsUpdatedDB int64 = 0
		for _, domainInBatch := range currentBatchDomains {
			domainID := domainInBatch.ID
			inputDomainName := domainInBatch.DomainName
			var domainUpdateData models.Domain

			if resultsForThisDomain, found := batchResultsMap[inputDomainName]; found && len(resultsForThisDomain) > 0 {
				bestResult := resultsForThisDomain[0]
				for k := 1; k < len(resultsForThisDomain); k++ {
					if isBetterResult(resultsForThisDomain[k], bestResult) {
						bestResult = resultsForThisDomain[k]
					}
				}
				domainUpdateData = models.Domain{
					ID:                domainID,
					HTTPStatusCode:    sql.NullInt64{Int64: int64(bestResult.StatusCode), Valid: bestResult.StatusCode != 0},
					HTTPContentLength: sql.NullInt64{Int64: bestResult.ContentLength, Valid: true},
					HTTPTitle:         models.NullString(bestResult.Title),
					HTTPServer:        models.NullString(bestResult.WebServer),
					HTTPTech:          models.NullString(strings.Join(bestResult.Technologies, ",")),
					HttpxFullJson:     models.NullString(bestResult.FullJson),
				}
				logger.Info("RunHttpxScan: Updating domain ID %d (input: %s) with httpx result.", domainID, inputDomainName)
			} else {
				if batchCmdErr == nil {
					logger.Info("RunHttpxScan: Domain '%s' (ID: %d) in batch yielded no direct results from httpx. Marking as scanned.", inputDomainName, domainID)
					noResultJson := `{"status_code": null, "message": "No HTTP response on probed ports"}`
					domainUpdateData = models.Domain{
						ID:            domainID,
						HttpxFullJson: models.NullString(noResultJson),
					}
				} else {
					logger.Info("RunHttpxScan: Skipping DB update for domain '%s' (ID: %d) due to batch httpx error.", inputDomainName, domainID)
					continue
				}
			}

			errDbUpdate := database.UpdateDomainWithHttpxResult(domainUpdateData)
			if errDbUpdate != nil {
				dbErrMsg := fmt.Sprintf("Failed to update domain ID %d (input: %s) with httpx result: %v", domainID, inputDomainName, errDbUpdate)
				logger.Error("RunHttpxScan: %s", dbErrMsg)
				httpxTaskStatusLock.Lock()
				if status, ok := httpxTaskStatuses[targetID]; ok {
					if status.LastError == "" {
						status.LastError = dbErrMsg
					}
				}
				httpxTaskStatusLock.Unlock()
			} else {
				batchDomainsUpdatedDB++
			}
		}
		cumulativeDomainsUpdatedDB += batchDomainsUpdatedDB

		httpxTaskStatusLock.Lock()
		if status, ok := httpxTaskStatuses[targetID]; ok {
			status.DomainsProcessed += batchDomainsProcessedThisIteration
			status.DomainsUpdatedDB = int(cumulativeDomainsUpdatedDB)
			status.Message = fmt.Sprintf("Batch %d of %d finished. Processed %d/%d domains. DB updates for %d domains.",
				(i/batchSize)+1, (numDomains+batchSize-1)/batchSize,
				status.DomainsProcessed, status.DomainsTotal, status.DomainsUpdatedDB)
		}
		httpxTaskStatusLock.Unlock()

		logMessageForBatch := fmt.Sprintf("Batch %d of %d finished for target ID %d. Domains attempted in batch: %d. DB updates for %d domains in batch. Cumulative DB updates: %d.",
			(i/batchSize)+1, (numDomains+batchSize-1)/batchSize, targetID,
			batchDomainsProcessedThisIteration, batchDomainsUpdatedDB, cumulativeDomainsUpdatedDB)
		logger.Info("RunHttpxScan: %s", logMessageForBatch)

	} // End of batch loop

	logger.Info("RunHttpxScan: Finished all httpx batches for target ID %d. Total domains attempted: %d. Total DB updates attempted: %d.", targetID, numDomains, cumulativeDomainsUpdatedDB)
}

// GetHttpxStatusHandler returns the current status of httpx scans for a target.
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
			StartTime: time.Now(),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// StopHttpxScanHandler handles requests to stop an ongoing httpx scan for a target.
func StopHttpxScanHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	targetID, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id", http.StatusBadRequest)
		return
	}

	httpxScanCancelFuncsLock.Lock()
	cancelFunc, ok := httpxScanCancelFuncs[targetID]
	// It's important to remove from the map *while holding the lock*
	// to prevent a race condition if Stop is called multiple times quickly
	// or if the scan finishes concurrently.
	if ok {
		delete(httpxScanCancelFuncs, targetID)
	}
	httpxScanCancelFuncsLock.Unlock()

	if ok && cancelFunc != nil {
		cancelFunc() // Call the context cancel function
		logger.Info("StopHttpxScanHandler: Cancellation requested for httpx scan on target ID %d.", targetID)

		// Update status immediately to reflect cancellation attempt
		httpxTaskStatusLock.Lock()
		if status, statOk := httpxTaskStatuses[targetID]; statOk {
			status.Message = "Httpx scan cancellation requested by user..."
			// IsRunning will be set to false by the RunHttpxScan's defer function when it exits.
		}
		httpxTaskStatusLock.Unlock()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Httpx scan cancellation initiated."})
	} else {
		logger.Warn("StopHttpxScanHandler: No active cancellable httpx scan found for target ID %d.", targetID)
		http.Error(w, "No active scan found to stop for this target.", http.StatusNotFound)
	}
}
