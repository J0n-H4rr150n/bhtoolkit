package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// CreateNoteHandler handles POST requests to create a new note.
func CreateNoteHandler(w http.ResponseWriter, r *http.Request) {
	var note models.Note
	if err := json.NewDecoder(r.Body).Decode(&note); err != nil {
		logger.Error("CreateNoteHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(note.Content) == "" {
		logger.Error("CreateNoteHandler: Note content cannot be empty.")
		http.Error(w, "Note content cannot be empty", http.StatusBadRequest)
		return
	}

	id, err := database.CreateNote(note)
	if err != nil {
		logger.Error("CreateNoteHandler: Error creating note: %v", err)
		http.Error(w, "Failed to create note", http.StatusInternalServerError)
		return
	}
	note.ID = id
	note.CreatedAt = time.Now()
	note.UpdatedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(note)
}

// GetNoteHandler handles GET requests for a single note by ID.
func GetNoteHandler(w http.ResponseWriter, r *http.Request, noteID int64) {
	note, err := database.GetNoteByID(noteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("GetNoteHandler: Note with ID %d not found", noteID)
			http.Error(w, "Note not found", http.StatusNotFound)
		} else {
			logger.Error("GetNoteHandler: Error fetching note %d: %v", noteID, err)
			http.Error(w, "Failed to retrieve note", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(note)
}

// ListNotesHandler handles GET requests to list all notes with pagination.
func ListNotesHandler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	sortBy := r.URL.Query().Get("sort_by")
	sortOrder := r.URL.Query().Get("sort_order")

	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if sortBy == "" {
		sortBy = "updated_at"
	}
	if sortOrder == "" || (strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC") {
		sortOrder = "DESC"
	}

	offset := (page - 1) * limit

	notes, totalRecords, err := database.GetAllNotesPaginated(limit, offset, sortBy, sortOrder)
	if err != nil {
		logger.Error("ListNotesHandler: Error fetching notes: %v", err)
		http.Error(w, "Failed to retrieve notes", http.StatusInternalServerError)
		return
	}

	totalPages := (totalRecords + int64(limit) - 1) / int64(limit)
	if totalPages == 0 && totalRecords > 0 {
		totalPages = 1
	}

	response := struct {
		Page         int           `json:"page"`
		Limit        int           `json:"limit"`
		TotalRecords int64         `json:"total_records"`
		TotalPages   int64         `json:"total_pages"`
		SortBy       string        `json:"sort_by"`
		SortOrder    string        `json:"sort_order"`
		Notes        []models.Note `json:"notes"`
	}{
		Page:         page,
		Limit:        limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
		SortBy:       sortBy,
		SortOrder:    sortOrder,
		Notes:        notes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateNoteHandler handles PUT requests to update an existing note.
func UpdateNoteHandler(w http.ResponseWriter, r *http.Request, noteID int64) {
	var updates models.Note
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		logger.Error("UpdateNoteHandler: Error decoding request body for note %d: %v", noteID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	existingNote, err := database.GetNoteByID(noteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Note not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve note for update", http.StatusInternalServerError)
		}
		return
	}

	existingNote.Title = updates.Title
	if strings.TrimSpace(updates.Content) == "" {
		http.Error(w, "Note content cannot be empty", http.StatusBadRequest)
		return
	}
	existingNote.Content = updates.Content

	err = database.UpdateNote(existingNote)
	if err != nil {
		logger.Error("UpdateNoteHandler: Error updating note %d: %v", noteID, err)
		http.Error(w, "Failed to update note", http.StatusInternalServerError)
		return
	}
	existingNote.UpdatedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existingNote)
}

// DeleteNoteHandler handles DELETE requests to remove a note.
func DeleteNoteHandler(w http.ResponseWriter, r *http.Request, noteID int64) {
	if _, err := database.GetNoteByID(noteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Note not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve note for deletion", http.StatusInternalServerError)
		}
		return
	}

	err := database.DeleteNote(noteID)
	if err != nil {
		logger.Error("DeleteNoteHandler: Error deleting note %d: %v", noteID, err)
		http.Error(w, "Failed to delete note", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
