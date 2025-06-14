package handlers

import (
	"encoding/json"
	"net/http"
	"toolkit/core" // Assuming your proxy core logic is here
	"toolkit/logger"

	"github.com/go-chi/chi/v5"
)

// RegisterProxySendRoutes registers the API routes related to sending requests via the proxy.
func RegisterProxySendRoutes(r chi.Router) {
	r.Post("/proxy/send-requests", SendPathsToProxyHandler)
}

// SendPathsRequest defines the expected structure for the request body
// for the SendPathsToProxyHandler.
type SendPathsRequest struct {
	TargetID int64    `json:"target_id"`
	URLs     []string `json:"urls"`
}

// SendPathsToProxyHandler handles requests to send a list of URLs through the proxy.
// It expects a JSON payload with target_id and an array of urls.
func SendPathsToProxyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("SendPathsToProxyHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SendPathsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("SendPathsToProxyHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.TargetID == 0 {
		logger.Error("SendPathsToProxyHandler: TargetID is required")
		http.Error(w, "TargetID is required", http.StatusBadRequest)
		return
	}
	if len(req.URLs) == 0 {
		logger.Error("SendPathsToProxyHandler: No URLs provided")
		http.Error(w, "No URLs provided to send", http.StatusBadRequest)
		return
	}

	// Go routine to prevent blocking the UI response, as sending requests can take time.
	go func() {
		err := core.SendGETRequestsThroughProxy(req.TargetID, req.URLs) // Call the core function
		if err != nil {
			logger.Error("Error sending requests through proxy via API: %v", err)
			// Error handling here is tricky as the HTTP response is already sent.
			// Log thoroughly. Maybe update a status in DB if needed for long-running tasks.
		} else {
			logger.Info("Successfully queued %d requests to be sent through proxy for target %d", len(req.URLs), req.TargetID)
		}
	}()

	// Respond immediately that the request was accepted for processing
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	json.NewEncoder(w).Encode(map[string]string{"message": "Requests accepted and are being sent to the proxy."})
}
