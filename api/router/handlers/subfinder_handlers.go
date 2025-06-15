package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"toolkit/logger"
)

// SubfinderStatusResponse defines the structure for the subfinder status.
type SubfinderStatusResponse struct {
	IsRunning             bool    `json:"is_running"`
	Message               string  `json:"message"`
	CompletedTasksSummary *string `json:"completed_tasks_summary,omitempty"` // Optional
}

// GetSubfinderStatusHandler handles requests to get the current subfinder status for a target.
// @Summary Get Subfinder Task Status
// @Description Retrieves the current status of background subfinder tasks for a specific target.
// @Tags Subfinder
// @Produce json
// @Param target_id query int true "ID of the target"
// @Success 200 {object} SubfinderStatusResponse
// @Failure 400 {object} models.ErrorResponse "Invalid or missing target_id"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /subfinder/status [get]
func GetSubfinderStatusHandler(w http.ResponseWriter, r *http.Request) {
	targetIDStr := r.URL.Query().Get("target_id")
	if targetIDStr == "" {
		http.Error(w, "target_id query parameter is required", http.StatusBadRequest)
		return
	}
	_, err := strconv.ParseInt(targetIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid target_id: must be an integer", http.StatusBadRequest)
		return
	}
	// Placeholder: In a real implementation, you would query the status of background tasks.
	response := SubfinderStatusResponse{IsRunning: false, Message: "No active subfinder tasks for this target."}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logger.Info("Served subfinder status for target_id: %s (placeholder response)", targetIDStr)
}
