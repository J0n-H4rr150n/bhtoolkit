package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"toolkit/logger"
)

func RegisterTrafficLogRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /traffic-log", GetTrafficLogHandler)
	mux.HandleFunc("/traffic-log/entry/", trafficLogEntryItemHandler) // Dispatcher
}

func trafficLogEntryItemHandler(w http.ResponseWriter, r *http.Request) {
	basePath := "/traffic-log/entry/"
	trimmedPath := strings.TrimPrefix(r.URL.Path, basePath)
	parts := strings.Split(strings.Trim(trimmedPath, "/"), "/")

	if len(parts) < 1 || parts[0] == "" {
		logger.Error("TrafficLogEntryItemHandler: Malformed path, missing log ID: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	logID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		logger.Error("TrafficLogEntryItemHandler: Invalid log entry ID '%s': %v", parts[0], err)
		http.Error(w, "Invalid log entry ID format", http.StatusBadRequest)
		return
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		getTrafficLogEntryDetail(w, r, logID)
	} else if len(parts) == 2 && parts[1] == "notes" && r.Method == http.MethodPut {
		updateTrafficLogEntryNotes(w, r, logID)
	} else if len(parts) == 2 && parts[1] == "favorite" && r.Method == http.MethodPut {
		setTrafficLogEntryFavoriteStatus(w, r, logID)
	} else {
		http.Error(w, "Method not allowed or path not found for traffic log entry", http.StatusMethodNotAllowed)
	}
}
