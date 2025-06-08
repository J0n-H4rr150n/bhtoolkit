package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// AddModifierTaskHandler handles requests to add a new task to the modifier.
func AddModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParameterizedURLID int64 `json:"parameterized_url_id"`
		// Potentially other ways to source a task, e.g., http_log_id or manual entry
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("AddModifierTaskHandler: Error decoding request: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.ParameterizedURLID == 0 {
		logger.Error("AddModifierTaskHandler: parameterized_url_id is required")
		http.Error(w, "parameterized_url_id is required", http.StatusBadRequest)
		return
	}

	// Fetch details from parameterized_urls table
	pURL, err := database.GetParameterizedURLByID(req.ParameterizedURLID) // Assumes this function exists
	if err != nil {
		logger.Error("AddModifierTaskHandler: Error fetching parameterized URL %d: %v", req.ParameterizedURLID, err)
		if err.Error() == fmt.Sprintf("parameterized URL with ID %d not found", req.ParameterizedURLID) {
			http.Error(w, "Parameterized URL not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve source URL details", http.StatusInternalServerError)
		}
		return
	}

	// Fetch the original log entry to get full headers and body
	originalLog, err := database.GetHTTPTrafficLogEntryByID(pURL.HTTPTrafficLogID) // Assumes this function exists
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("AddModifierTaskHandler: Original HTTP log entry %d (referenced by pURL %d) not found: %v", pURL.HTTPTrafficLogID, pURL.ID, err)
			http.Error(w, "Original HTTP log entry not found", http.StatusNotFound)
		} else {
			logger.Error("AddModifierTaskHandler: Error fetching original log entry %d for pURL %d: %v", pURL.HTTPTrafficLogID, pURL.ID, err)
			http.Error(w, "Failed to retrieve original request details", http.StatusInternalServerError)
		}
		return
	}

	taskName := fmt.Sprintf("%s %s (from pURL %d)", pURL.RequestMethod, pURL.RequestPath, pURL.ID)

	modifierTask := models.ModifierTask{
		TargetID:           sql.NullInt64{Int64: pURL.TargetID, Valid: true},
		ParameterizedURLID: sql.NullInt64{Int64: pURL.ID, Valid: true},
		OriginalLogID:      sql.NullInt64{Int64: pURL.HTTPTrafficLogID, Valid: true},
		Name:               taskName,
		BaseRequestMethod:  originalLog.RequestMethod,
		BaseRequestURL:     originalLog.RequestURL,     // Full original URL
		BaseRequestHeaders: originalLog.RequestHeaders, // Assuming this is already a JSON string
		BaseRequestBody:    base64.StdEncoding.EncodeToString(originalLog.RequestBody),
		Notes:              "",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	taskID, err := database.CreateModifierTask(modifierTask)
	if err != nil {
		logger.Error("AddModifierTaskHandler: Error creating modifier task: %v", err)
		http.Error(w, "Failed to create modifier task", http.StatusInternalServerError)
		return
	}

	modifierTask.ID = taskID // Set the ID on the task object for the response

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(modifierTask)
	logger.Info("Modifier task created with ID %d from pURL ID %d", taskID, pURL.ID)
}

// GetModifierTasksHandler retrieves a list of modifier tasks.
// Supports optional 'target_id' query parameter for filtering.
func GetModifierTasksHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	var targetIDFilter sql.NullInt64

	if targetIDStr != "" {
		id, err := strconv.ParseInt(targetIDStr, 10, 64)
		if err != nil {
			logger.Error("GetModifierTasksHandler: Invalid target_id query parameter '%s': %v", targetIDStr, err)
			http.Error(w, "Invalid target_id query parameter", http.StatusBadRequest)
			return
		}
		targetIDFilter = sql.NullInt64{Int64: id, Valid: true}
	}

	tasks, err := database.GetModifierTasks(targetIDFilter)
	if err != nil {
		logger.Error("GetModifierTasksHandler: Error fetching modifier tasks: %v", err)
		http.Error(w, "Failed to get tasks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// GetModifierTaskDetailsHandler retrieves details for a specific modifier task.
func GetModifierTaskDetailsHandler(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		logger.Error("GetModifierTaskDetailsHandler: Invalid task_id '%s': %v", taskIDStr, err)
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	task, err := database.GetModifierTaskByID(taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("GetModifierTaskDetailsHandler: Modifier task with ID %d not found: %v", taskID, err)
			http.Error(w, "Modifier task not found", http.StatusNotFound)
		} else {
			logger.Error("GetModifierTaskDetailsHandler: Error fetching task %d: %v", taskID, err)
			http.Error(w, "Failed to retrieve task details", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// ExecuteModifiedRequestHandler handles sending a modified HTTP request.
func ExecuteModifiedRequestHandler(w http.ResponseWriter, r *http.Request) {
	var reqPayload struct {
		TaskID  int64  `json:"task_id"` // Optional, for future versioning
		Method  string `json:"method"`
		URL     string `json:"url"`
		Headers string `json:"headers"` // Newline-separated "Key: Value"
		Body    string `json:"body"`    // Plain text body
	}

	if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error decoding request: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if reqPayload.Method == "" || reqPayload.URL == "" {
		logger.Error("ExecuteModifiedRequestHandler: Method and URL are required")
		http.Error(w, "Method and URL are required", http.StatusBadRequest)
		return
	}

	// Construct the HTTP request
	httpRequest, err := http.NewRequest(strings.ToUpper(reqPayload.Method), reqPayload.URL, strings.NewReader(reqPayload.Body))
	if err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error creating new request: %v", err)
		http.Error(w, fmt.Sprintf("Error creating request: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse and set headers
	// Header string is "Key1: Value1\nKey2: Value2"
	headerLines := strings.Split(reqPayload.Headers, "\n")
	for _, line := range headerLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// For http.Request.Header, some headers like "Host" are special
			// and should be set on httpRequest.Host directly.
			// For simplicity now, we set all via Header.Add().
			// Chi's reverse proxy or a more custom client might handle Host correctly.
			if strings.ToLower(key) == "host" {
				httpRequest.Host = value
			} else {
				httpRequest.Header.Add(key, value)
			}
		}
	}

	// TODO: Consider using a custom http.Client with options for timeouts, proxy, SSL verification skip.
	// For now, use the default client.
	client := &http.Client{
		// Example: Disable redirects if needed for testing
		// CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// 	return http.ErrUseLastResponse
		// },
	}
	httpResponse, err := client.Do(httpRequest)
	if err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error sending request: %v", err)
		http.Error(w, fmt.Sprintf("Error sending request: %v", err), http.StatusInternalServerError)
		return
	}
	defer httpResponse.Body.Close()

	responseBodyBytes, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		logger.Error("ExecuteModifiedRequestHandler: Error reading response body: %v", err)
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}

	responsePayload := map[string]interface{}{
		"status_code": httpResponse.StatusCode,
		"status_text": httpResponse.Status,
		"headers":     httpResponse.Header,                                  // This is map[string][]string
		"body":        base64.StdEncoding.EncodeToString(responseBodyBytes), // Send body as base64
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responsePayload)
}

// UpdateModifierTaskHandler handles requests to update a modifier task (e.g., its name).
func UpdateModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskIDStr := chi.URLParam(r, "task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		logger.Error("UpdateModifierTaskHandler: Invalid task_id '%s': %v", taskIDStr, err)
		http.Error(w, "Invalid task ID", http.StatusBadRequest)
		return
	}

	var reqPayload struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
		logger.Error("UpdateModifierTaskHandler: Error decoding request body for task ID %d: %v", taskID, err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	newName := strings.TrimSpace(reqPayload.Name)
	if newName == "" {
		logger.Error("UpdateModifierTaskHandler: Task name cannot be empty for task ID %d", taskID)
		http.Error(w, "Task name cannot be empty", http.StatusBadRequest)
		return
	}

	updatedTask, err := database.UpdateModifierTaskName(taskID, newName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.Error("UpdateModifierTaskHandler: Task with ID %d not found for update: %v", taskID, err)
			http.Error(w, "Task not found", http.StatusNotFound)
		} else {
			logger.Error("UpdateModifierTaskHandler: Error updating task name for ID %d: %v", taskID, err)
			http.Error(w, "Failed to update task name", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedTask)
	logger.Info("Modifier task ID %d updated successfully. New name: '%s'", taskID, updatedTask.Name)
}

// CloneModifierTaskHandler handles requests to clone an existing modifier task.
func CloneModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	originalTaskIDStr := chi.URLParam(r, "task_id")
	originalTaskID, err := strconv.ParseInt(originalTaskIDStr, 10, 64)
	if err != nil {
		logger.Error("CloneModifierTaskHandler: Invalid original_task_id '%s': %v", originalTaskIDStr, err)
		http.Error(w, "Invalid original task ID", http.StatusBadRequest)
		return
	}

	// Fetch the original task
	originalTask, err := database.GetModifierTaskByID(originalTaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
			logger.Error("CloneModifierTaskHandler: Original task with ID %d not found for cloning: %v", originalTaskID, err)
			http.Error(w, "Original task not found", http.StatusNotFound)
		} else {
			logger.Error("CloneModifierTaskHandler: Error fetching original task ID %d: %v", originalTaskID, err)
			http.Error(w, "Failed to retrieve original task details", http.StatusInternalServerError)
		}
		return
	}

	// Create the new task based on the original
	clonedTask := originalTask // Copy all fields
	clonedTask.ID = 0          // Reset ID to indicate a new task
	clonedTask.Name = originalTask.Name + " (Clone)"
	clonedTask.CreatedAt = time.Now()
	clonedTask.UpdatedAt = time.Now()

	newID, err := database.CreateModifierTask(clonedTask) // CreateModifierTask should handle inserting a new record
	if err != nil {
		logger.Error("CloneModifierTaskHandler: Error creating cloned task from original ID %d: %v", originalTaskID, err)
		http.Error(w, "Failed to clone task", http.StatusInternalServerError)
		return
	}
	clonedTask.ID = newID // Set the new ID

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(clonedTask)
	logger.Info("Modifier task ID %d cloned successfully as new task ID %d", originalTaskID, newID)
}

// UpdateModifierTasksOrderHandler handles requests to update the display order of multiple modifier tasks.
func UpdateModifierTasksOrderHandler(w http.ResponseWriter, r *http.Request) {
	var taskOrders map[string]int // Expecting a map like {"taskID_string": order_int}
	// Alternatively, could be []struct { ID int64 `json:"id"`; Order int `json:"order"` }

	if err := json.NewDecoder(r.Body).Decode(&taskOrders); err != nil {
		logger.Error("UpdateModifierTasksOrderHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(taskOrders) == 0 {
		// It's not an error to send an empty list, just means no changes.
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "No task orders provided to update."})
		return
	}

	dbTaskOrders := make(map[int64]int)
	for idStr, order := range taskOrders {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			logger.Error("UpdateModifierTasksOrderHandler: Invalid task ID '%s' in request: %v", idStr, err)
			http.Error(w, fmt.Sprintf("Invalid task ID '%s' in request", idStr), http.StatusBadRequest)
			return
		}
		dbTaskOrders[id] = order
	}

	if err := database.UpdateModifierTasksOrder(dbTaskOrders); err != nil {
		logger.Error("UpdateModifierTasksOrderHandler: Error updating task orders in database: %v", err)
		http.Error(w, "Failed to update task orders", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Task orders updated successfully."})
	logger.Info("Successfully processed request to update display order for %d modifier tasks.", len(dbTaskOrders))
}
