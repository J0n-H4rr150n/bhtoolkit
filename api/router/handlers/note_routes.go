package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func RegisterNoteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notes" { // Ensure exact match for collection
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			ListNotesHandler(w, r)
		case http.MethodPost:
			CreateNoteHandler(w, r)
		default:
			http.Error(w, "Method not allowed for /notes", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/notes/", func(w http.ResponseWriter, r *http.Request) {
		idStr := strings.TrimPrefix(r.URL.Path, "/notes/")
		noteID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid note ID format", http.StatusBadRequest)
			return
		}
		handleSingleNoteRequest(w, r, noteID)
	})
}

func handleSingleNoteRequest(w http.ResponseWriter, r *http.Request, noteID int64) {
	switch r.Method {
	case http.MethodGet:
		GetNoteHandler(w, r, noteID)
	case http.MethodPut:
		UpdateNoteHandler(w, r, noteID)
	case http.MethodDelete:
		DeleteNoteHandler(w, r, noteID)
	default:
		http.Error(w, fmt.Sprintf("Method not allowed for /notes/%d", noteID), http.StatusMethodNotAllowed)
	}
}
