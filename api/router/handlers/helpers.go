package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"toolkit/models" // For ErrorResponse
)

// notImplementedHandler returns a 501 Not Implemented error.
func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	errMsg := fmt.Sprintf("%s %s - Not Implemented Yet (relative path within API router)", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	// Ensure models.ErrorResponse is used if you have a specific structure
	json.NewEncoder(w).Encode(models.ErrorResponse{Message: errMsg})
}
