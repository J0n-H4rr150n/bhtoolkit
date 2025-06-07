package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// GetChecklistItemsHandler retrieves all checklist items for a given target.
func GetChecklistItemsHandler(w http.ResponseWriter, r *http.Request, targetID int64) {
	if r.Method != http.MethodGet {
		logger.Error("GetChecklistItemsHandler: MethodNotAllowed: %s for target %d", r.Method, targetID)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	items, err := database.GetChecklistItemsByTargetID(targetID)
	if err != nil {
		logger.Error("GetChecklistItemsHandler: Error fetching checklist items for target %d: %v", targetID, err)
		http.Error(w, "Failed to retrieve checklist items", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// AddChecklistItemHandler adds a new checklist item for a target.
func AddChecklistItemHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("AddChecklistItemHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestPayload struct {
		TargetID        int64   `json:"target_id"`
		ItemText        string  `json:"item_text"`
		ItemCommandText *string `json:"item_command_text"`
		Notes           *string `json:"notes"`
		IsCompleted     bool    `json:"is_completed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
		logger.Error("AddChecklistItemHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var item models.TargetChecklistItem
	item.TargetID = requestPayload.TargetID
	item.ItemText = requestPayload.ItemText
	item.IsCompleted = requestPayload.IsCompleted
	if requestPayload.ItemCommandText != nil {
		item.ItemCommandText = sql.NullString{String: *requestPayload.ItemCommandText, Valid: true}
	}
	if requestPayload.Notes != nil {
		item.Notes = sql.NullString{String: *requestPayload.Notes, Valid: true}
	}

	if item.TargetID == 0 || strings.TrimSpace(item.ItemText) == "" {
		logger.Error("AddChecklistItemHandler: TargetID and ItemText are required. Got TargetID: %d, ItemText: '%s'", item.TargetID, item.ItemText)
		http.Error(w, "TargetID and ItemText are required", http.StatusBadRequest)
		return
	}

	id, err := database.AddChecklistItem(item)
	if err != nil {
		logger.Error("AddChecklistItemHandler: Error adding checklist item for target %d: %v", item.TargetID, err)
		http.Error(w, "Failed to add checklist item", http.StatusInternalServerError)
		return
	}
	item.ID = id
	item.CreatedAt = time.Now()
	item.UpdatedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

// UpdateChecklistItemHandler updates an existing checklist item.
func UpdateChecklistItemHandler(w http.ResponseWriter, r *http.Request, itemID int64) {
	if r.Method != http.MethodPut {
		logger.Error("UpdateChecklistItemHandler: MethodNotAllowed: %s for item %d", r.Method, itemID)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var itemUpdates struct {
		ItemText        *string `json:"item_text"`
		ItemCommandText *string `json:"item_command_text"`
		Notes           *string `json:"notes"`
		IsCompleted     *bool   `json:"is_completed"`
	}

	bodyBytes, bodyReadErr := io.ReadAll(r.Body)
	if bodyReadErr != nil {
		logger.Error("UpdateChecklistItemHandler: Error reading request body for item %d: %v", itemID, bodyReadErr)
		http.Error(w, "Failed to read request body: "+bodyReadErr.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	if err := json.Unmarshal(bodyBytes, &itemUpdates); err != nil {
		logger.Error("UpdateChecklistItemHandler: Error decoding request body for item %d: %v", itemID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	existingItem, err := database.GetChecklistItemByID(itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("UpdateChecklistItemHandler: Checklist item with ID %d not found", itemID)
			http.Error(w, "Checklist item not found", http.StatusNotFound)
		} else {
			logger.Error("UpdateChecklistItemHandler: Error fetching checklist item %d: %v", itemID, err)
			http.Error(w, "Failed to retrieve checklist item", http.StatusInternalServerError)
		}
		return
	}

	if itemUpdates.ItemText != nil {
		existingItem.ItemText = *itemUpdates.ItemText
	}

	var rawRequestBody map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &rawRequestBody) // Error already handled, this is for key check

	if _, keyExists := rawRequestBody["item_command_text"]; keyExists {
		if itemUpdates.ItemCommandText == nil {
			existingItem.ItemCommandText = sql.NullString{String: "", Valid: false}
		} else {
			existingItem.ItemCommandText = sql.NullString{String: *itemUpdates.ItemCommandText, Valid: true}
		}
	}

	if _, keyExists := rawRequestBody["notes"]; keyExists {
		if itemUpdates.Notes == nil {
			existingItem.Notes = sql.NullString{String: "", Valid: false}
		} else {
			existingItem.Notes = sql.NullString{String: *itemUpdates.Notes, Valid: true}
		}
	}

	if itemUpdates.IsCompleted != nil {
		existingItem.IsCompleted = *itemUpdates.IsCompleted
	}

	if strings.TrimSpace(existingItem.ItemText) == "" {
		logger.Error("UpdateChecklistItemHandler: ItemText cannot be empty for item %d", itemID)
		http.Error(w, "ItemText cannot be empty", http.StatusBadRequest)
		return
	}

	err = database.UpdateChecklistItem(existingItem)
	if err != nil {
		logger.Error("UpdateChecklistItemHandler: Error updating checklist item %d: %v", itemID, err)
		http.Error(w, "Failed to update checklist item", http.StatusInternalServerError)
		return
	}
	existingItem.UpdatedAt = time.Now()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existingItem)
}

// DeleteChecklistItemHandler deletes a checklist item.
func DeleteChecklistItemHandler(w http.ResponseWriter, r *http.Request, itemID int64) {
	if r.Method != http.MethodDelete {
		logger.Error("DeleteChecklistItemHandler: MethodNotAllowed: %s for item %d", r.Method, itemID)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, err := database.GetChecklistItemByID(itemID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("DeleteChecklistItemHandler: Checklist item with ID %d not found for deletion", itemID)
			http.Error(w, "Checklist item not found", http.StatusNotFound)
		} else {
			logger.Error("DeleteChecklistItemHandler: Error fetching checklist item %d before deletion: %v", itemID, err)
			http.Error(w, "Failed to retrieve checklist item for deletion", http.StatusInternalServerError)
		}
		return
	}

	err = database.DeleteChecklistItem(itemID)
	if err != nil {
		logger.Error("DeleteChecklistItemHandler: Error deleting checklist item %d: %v", itemID, err)
		http.Error(w, "Failed to delete checklist item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CopyTemplateItemsToTargetRequest represents the request body for copying items.
type CopyTemplateItemsToTargetRequest struct {
	TargetID int64 `json:"target_id"`
	Items    []struct {
		ItemText        string `json:"item_text"`
		ItemCommandText string `json:"item_command_text"`
		Notes           string `json:"notes"`
	} `json:"items"`
}

// CopyTemplateItemsToTargetResponse represents the response body after copying items.
type CopyTemplateItemsToTargetResponse struct {
	CopiedCount  int    `json:"copied_count"`
	SkippedCount int    `json:"skipped_count"`
	Message      string `json:"message"`
}

// CopyTemplateItemsToTargetHandler handles the request to copy checklist template items to a target.
func CopyTemplateItemsToTargetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("CopyTemplateItemsToTargetHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed. Only POST is accepted.", http.StatusMethodNotAllowed)
		return
	}

	var req CopyTemplateItemsToTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("CopyTemplateItemsToTargetHandler: Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.TargetID <= 0 {
		logger.Error("CopyTemplateItemsToTargetHandler: Invalid target_id: %d", req.TargetID)
		http.Error(w, "Invalid target_id", http.StatusBadRequest)
		return
	}

	copiedCount := 0
	skippedCount := 0
	var errorMessages []string

	for _, item := range req.Items {
		notes := models.NullString(item.Notes)
		commandText := models.NullString(item.ItemCommandText)

		_, inserted, err := database.AddChecklistItemIfNotExists(req.TargetID, item.ItemText, commandText, notes)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to add item '%s': %v", item.ItemText, err)
			logger.Error("CopyTemplateItemsToTargetHandler: %s for target %d", errorMsg, req.TargetID)
			errorMessages = append(errorMessages, errorMsg)
		} else if inserted {
			copiedCount++
		} else {
			skippedCount++
		}
	}

	message := fmt.Sprintf("Operation complete: %d item(s) copied, %d item(s) skipped.", copiedCount, skippedCount)
	if len(errorMessages) > 0 {
		message += " Errors: " + strings.Join(errorMessages, "; ")
	}

	resp := CopyTemplateItemsToTargetResponse{
		CopiedCount:  copiedCount,
		SkippedCount: skippedCount,
		Message:      message,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	logger.Info("CopyTemplateItemsToTargetHandler: For target %d, copied %d, skipped %d items. Errors: %d", req.TargetID, copiedCount, skippedCount, len(errorMessages))
}
