package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func RegisterChecklistRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/checklist-items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			AddChecklistItemHandler(w, r)
		} else {
			http.Error(w, "Method not allowed for /checklist-items", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/checklist-items/", func(w http.ResponseWriter, r *http.Request) {
		itemIDStr := strings.TrimPrefix(r.URL.Path, "/checklist-items/")
		itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid checklist item ID", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPut:
			UpdateChecklistItemHandler(w, r, itemID)
		case http.MethodDelete:
			DeleteChecklistItemHandler(w, r, itemID)
		default:
			http.Error(w, fmt.Sprintf("Method not allowed for /checklist-items/%d", itemID), http.StatusMethodNotAllowed)
		}
	})
}
