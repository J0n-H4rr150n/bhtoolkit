package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/go-chi/chi/v5"
)

// updateTagPayload defines the expected structure for updating a tag.
type updateTagPayload struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"` // Pointer for optional update, allows "" to clear or null to not change
}

// UpdateTagHandler handles PUT requests to update an existing tag.
func UpdateTagHandler(w http.ResponseWriter, r *http.Request) {
	tagIDStr := chi.URLParam(r, "tagID")
	tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
	if err != nil {
		logger.Error("UpdateTagHandler: Invalid tag ID format '%s': %v", tagIDStr, err)
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}

	var payload updateTagPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("UpdateTagHandler: Error decoding request body for tag %d: %v", tagID, err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Fetch existing tag to ensure it exists and to apply partial updates
	_, err = database.GetTagByID(tagID) // Check for existence
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("UpdateTagHandler: Tag with ID %d not found", tagID)
			http.Error(w, "Tag not found", http.StatusNotFound)
		} else {
			logger.Error("UpdateTagHandler: Error fetching tag %d for update: %v", tagID, err)
			http.Error(w, "Failed to retrieve tag for update", http.StatusInternalServerError)
		}
		return
	}

	// Construct the models.Tag for the database.UpdateTag function
	tagForDBUpdate := models.Tag{ID: tagID}

	if payload.Name != nil {
		trimmedName := strings.TrimSpace(*payload.Name)
		if trimmedName == "" {
			logger.Error("UpdateTagHandler: Tag name cannot be empty if provided for update (tag ID %d)", tagID)
			http.Error(w, "Tag name cannot be empty if provided", http.StatusBadRequest)
			return
		}
		tagForDBUpdate.Name = trimmedName
	}

	if payload.Color != nil { // If "color" key is present in JSON (even if value is empty string)
		tagForDBUpdate.Color = sql.NullString{String: *payload.Color, Valid: true}
	} else { // If "color" key is not in JSON, Color.Valid will be false, and DB won't update it.
		tagForDBUpdate.Color = sql.NullString{Valid: false}
	}

	updatedTag, err := database.UpdateTag(tagForDBUpdate)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: tags.name") && payload.Name != nil {
			logger.Error("UpdateTagHandler: Tag name '%s' already exists (tag ID %d): %v", *payload.Name, tagID, err)
			http.Error(w, fmt.Sprintf("Tag name '%s' already exists.", *payload.Name), http.StatusConflict)
		} else {
			logger.Error("UpdateTagHandler: Error updating tag %d: %v", tagID, err)
			http.Error(w, "Failed to update tag", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(updatedTag)
	logger.Info("Successfully updated tag ID %d. New name: '%s', New color: '%s'", updatedTag.ID, updatedTag.Name, updatedTag.Color.String)
}

// DeleteTagHandler handles DELETE requests to remove a tag.
func DeleteTagHandler(w http.ResponseWriter, r *http.Request) {
	tagIDStr := chi.URLParam(r, "tagID")
	tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
	if err != nil {
		logger.Error("DeleteTagHandler: Invalid tag ID format '%s': %v", tagIDStr, err)
		http.Error(w, "Invalid tag ID", http.StatusBadRequest)
		return
	}

	// First, check if the tag exists
	_, err = database.GetTagByID(tagID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("DeleteTagHandler: Tag with ID %d not found for deletion", tagID)
			http.Error(w, "Tag not found", http.StatusNotFound)
		} else {
			logger.Error("DeleteTagHandler: Error fetching tag %d before deletion: %v", tagID, err)
			http.Error(w, "Failed to retrieve tag for deletion", http.StatusInternalServerError)
		}
		return
	}

	if err := database.DeleteTagAndAssociations(tagID); err != nil {
		logger.Error("DeleteTagHandler: Error deleting tag %d and its associations: %v", tagID, err)
		http.Error(w, "Failed to delete tag", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content is typical for successful DELETE
	logger.Info("Successfully deleted tag ID %d and its associations.", tagID)
}

// ListTagsHandler handles GET requests to list all tags.
// (Currently a stub)
func ListTagsHandler(w http.ResponseWriter, r *http.Request) {
	tags, err := database.GetAllTags()
	if err != nil {
		logger.Error("ListTagsHandler: Error fetching all tags: %v", err)
		http.Error(w, "Failed to retrieve tags", http.StatusInternalServerError)
		return
	}
	if tags == nil {
		tags = []models.Tag{} // Return empty array instead of null
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tags)
	logger.Info("ListTagsHandler: Successfully served %d tags.", len(tags))
}

// createTagPayload defines the expected structure for creating a tag.
type createTagPayload struct {
	Name  string  `json:"name"`
	Color *string `json:"color"` // Use pointer to handle optional color
}

// CreateTagHandler handles POST requests to create a new tag.
func CreateTagHandler(w http.ResponseWriter, r *http.Request) {
	var payload createTagPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("CreateTagHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	trimmedName := strings.TrimSpace(payload.Name)
	if trimmedName == "" {
		logger.Error("CreateTagHandler: Tag name cannot be empty.")
		http.Error(w, "Tag name cannot be empty", http.StatusBadRequest)
		return
	}
	tagToCreate := models.Tag{Name: trimmedName}
	if payload.Color != nil {
		tagToCreate.Color = sql.NullString{String: *payload.Color, Valid: true}
	}
	createdTag, err := database.CreateTag(tagToCreate) // CreateTag handles if it already exists by name
	if err != nil {
		// CreateTag already logs specific errors like UNIQUE constraint
		http.Error(w, "Failed to create tag: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated) // 201 Created (or 200 OK if it might return existing)
	json.NewEncoder(w).Encode(createdTag)
	logger.Info("CreateTagHandler: Successfully created/retrieved tag ID %d with name '%s'", createdTag.ID, createdTag.Name)
}

// GetTagByIDHandler handles GET requests for a specific tag by its ID.
// (Currently a stub)
func GetTagByIDHandler(w http.ResponseWriter, r *http.Request) {
	// tagIDStr := chi.URLParam(r, "tagID")
	// TODO: Implement logic to fetch and return tag by ID
	logger.Info("GetTagByIDHandler: Called (Not Implemented Yet)")
	notImplementedHandler(w, r)
}

// associateTagPayload defines the expected structure for associating a tag.
type associateTagPayload struct {
	TagID    int64  `json:"tag_id"`
	ItemID   int64  `json:"item_id"`
	ItemType string `json:"item_type"`
}

// AssociateTagHandler handles POST requests to associate a tag with an item.
func AssociateTagHandler(w http.ResponseWriter, r *http.Request) {
	var payload associateTagPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("AssociateTagHandler: Error decoding request body: %v", err)
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	payload.ItemType = strings.TrimSpace(strings.ToLower(payload.ItemType))
	if payload.TagID == 0 || payload.ItemID == 0 || payload.ItemType == "" {
		logger.Error("AssociateTagHandler: tag_id, item_id, and item_type are required. Got TagID: %d, ItemID: %d, ItemType: '%s'", payload.TagID, payload.ItemID, payload.ItemType)
		http.Error(w, "tag_id, item_id, and item_type are required", http.StatusBadRequest)
		return
	}

	association, err := database.AssociateTagWithItem(payload.TagID, payload.ItemID, payload.ItemType)
	if err != nil {
		// database.AssociateTagWithItem logs specific errors
		http.Error(w, "Failed to associate tag: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(association)
	logger.Info("AssociateTagHandler: Successfully associated tag %d with item %d (type: %s)", payload.TagID, payload.ItemID, payload.ItemType)
}

// DisassociateTagHandler handles DELETE requests to remove a tag association.
func DisassociateTagHandler(w http.ResponseWriter, r *http.Request) {
	tagIDStr := r.URL.Query().Get("tag_id")
	itemIDStr := r.URL.Query().Get("item_id")
	itemType := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("item_type")))

	if tagIDStr == "" || itemIDStr == "" || itemType == "" {
		logger.Error("DisassociateTagHandler: tag_id, item_id, and item_type query parameters are required.")
		http.Error(w, "tag_id, item_id, and item_type query parameters are required", http.StatusBadRequest)
		return
	}

	tagID, err := strconv.ParseInt(tagIDStr, 10, 64)
	if err != nil {
		logger.Error("DisassociateTagHandler: Invalid tag_id format '%s': %v", tagIDStr, err)
		http.Error(w, "Invalid tag_id format", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		logger.Error("DisassociateTagHandler: Invalid item_id format '%s': %v", itemIDStr, err)
		http.Error(w, "Invalid item_id format", http.StatusBadRequest)
		return
	}

	if err := database.RemoveTagAssociation(tagID, itemID, itemType); err != nil {
		// database.RemoveTagAssociation logs specific errors
		http.Error(w, "Failed to remove tag association: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content is typical for successful DELETE
	logger.Info("DisassociateTagHandler: Successfully removed association for tag %d from item %d (type: %s)", tagID, itemID, itemType)
}
