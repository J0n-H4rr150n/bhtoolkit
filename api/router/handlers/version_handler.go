package handlers

import (
	"encoding/json"
	"net/http"
	"toolkit/version" // Import the new version package
)

// GetVersionHandler returns the application version.
// @Summary Get application version
// @Description Retrieves the current version of the application.
// @Tags Version
// @Produce json
// @Success 200 {object} map[string]string "{"version": "1.0.0"}"
// @Router /version [get]
func GetVersionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": version.AppVersion})
}
