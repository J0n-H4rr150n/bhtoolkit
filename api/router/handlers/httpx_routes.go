package handlers

import (
	"github.com/go-chi/chi/v5"
)

// RegisterHttpxRoutes sets up routes specific to httpx operations.
func RegisterHttpxRoutes(r chi.Router) {
	r.Get("/httpx/status", GetHttpxStatusHandler) // Existing
	r.Post("/httpx/stop", StopHttpxScanHandler)   // New route to stop a scan
}
