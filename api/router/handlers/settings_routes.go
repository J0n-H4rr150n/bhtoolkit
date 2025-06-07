package handlers

import (
	"net/http"
)

func RegisterSettingsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /settings/current-target", GetCurrentTargetSettingHandler)
	mux.HandleFunc("POST /settings/current-target", SetCurrentTargetSettingHandler)

	mux.HandleFunc("/settings/custom-headers", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/settings/custom-headers" { // Ensure exact match
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			GetCustomHeadersSettingHandler(w, r)
		case http.MethodPost, http.MethodPut:
			SetCustomHeadersSettingHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/settings/table-column-widths", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/settings/table-column-widths" { // Ensure exact match
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			GetTableColumnWidthsHandler(w, r)
		case http.MethodPost, http.MethodPut:
			SetTableColumnWidthsHandler(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("GET /ui-settings", GetUISettingsHandler)
}
