package handlers

import (
	"encoding/json"
	"net/http"
	"toolkit/logger"

	"github.com/go-chi/chi/v5"
)

func RegisterHealthRoutes(r chi.Router) {
	r.Get("/health", healthCheckHandler)
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]bool{"ok": true}); err != nil {
		logger.Error("Error encoding health check response: %v", err)
	}
}
