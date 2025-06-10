package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

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

// ExecuteModifiedRequestHandler handles executing a modified request.
func ExecuteModifiedRequestHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement logic to parse request, send it, and return response
	logger.Info("ExecuteModifiedRequestHandler called - NOT IMPLEMENTED")
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// UpdateModifierTaskHandler handles updating parts of a task, like its name.
func UpdateModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement logic to parse task_id and update payload, then call database
	logger.Info("UpdateModifierTaskHandler called - NOT IMPLEMENTED")
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// CloneModifierTaskHandler handles cloning an existing modifier task.
func CloneModifierTaskHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement logic to get task_id, fetch original, create new
	logger.Info("CloneModifierTaskHandler called - NOT IMPLEMENTED")
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

// UpdateModifierTasksOrderHandler handles updating the display order of modifier tasks.
func UpdateModifierTasksOrderHandler(w http.ResponseWriter, r *http.Request) {
	var taskOrders map[string]int // Expecting {"taskID_string": order_int}
	if err := json.NewDecoder(r.Body).Decode(&taskOrders); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	// TODO: Convert map keys to int64 and call database.UpdateModifierTasksOrder
	logger.Info("UpdateModifierTasksOrderHandler called with %d orders - DB call NOT IMPLEMENTED", len(taskOrders))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Task order update received (processing not fully implemented)."})
}
