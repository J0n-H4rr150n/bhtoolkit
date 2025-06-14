package handlers

import (
	"encoding/json"
	"fmt" // Added for formatting messages
	"net/http"
	"strconv"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"
)

// ListChecklistTemplatesHandler handles GET requests to list all checklist templates.
func ListChecklistTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		logger.Error("ListChecklistTemplatesHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templates, err := database.GetAllChecklistTemplates()
	if err != nil {
		logger.Error("ListChecklistTemplatesHandler: Error fetching checklist templates: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(templates); err != nil {
		logger.Error("ListChecklistTemplatesHandler: Error encoding response: %v", err)
	}
	logger.Info("Successfully fetched %d checklist templates.", len(templates))
}

// GetChecklistTemplateItemsHandler handles GET requests for items of a specific checklist template.
func GetChecklistTemplateItemsHandler(w http.ResponseWriter, r *http.Request, templateID int64) {
	if r.Method != http.MethodGet {
		logger.Error("GetChecklistTemplateItemsHandler: MethodNotAllowed: %s for template ID %d", r.Method, templateID)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 20
	} else if limit > 200 {
		limit = 200
	}
	offset := (page - 1) * limit

	items, totalRecords, err := database.GetChecklistTemplateItemsPaginated(templateID, limit, offset)
	if err != nil {
		logger.Error("GetChecklistTemplateItemsHandler: Error fetching items for template ID %d: %v", templateID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	totalPages := (totalRecords + int64(limit) - 1) / int64(limit)
	if totalPages == 0 && totalRecords > 0 {
		totalPages = 1
	}

	response := models.PaginatedChecklistTemplateItemsResponse{
		Page:         page,
		Limit:        limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
		Items:        items,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("GetChecklistTemplateItemsHandler: Error encoding response for template ID %d: %v", templateID, err)
	}
	logger.Info("Successfully fetched %d items for checklist template ID %d (Page %d, Total %d).", len(items), templateID, page, totalRecords)
}

// CopyAllChecklistTemplateItemsToTargetHandler handles requests to copy all items from a template to a target.
func CopyAllChecklistTemplateItemsToTargetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		logger.Error("CopyAllChecklistTemplateItemsToTargetHandler: MethodNotAllowed: %s", r.Method)
		http.Error(w, "Method not allowed. Only POST is accepted.", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TemplateID int64 `json:"template_id"`
		TargetID   int64 `json:"target_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("CopyAllChecklistTemplateItemsToTargetHandler: Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if req.TemplateID <= 0 {
		logger.Error("CopyAllChecklistTemplateItemsToTargetHandler: Invalid template_id: %d", req.TemplateID)
		http.Error(w, "Invalid template_id", http.StatusBadRequest)
		return
	}
	if req.TargetID <= 0 {
		logger.Error("CopyAllChecklistTemplateItemsToTargetHandler: Invalid target_id: %d", req.TargetID)
		http.Error(w, "Invalid target_id", http.StatusBadRequest)
		return
	}

	itemsCopied, err := database.CopyAllTemplateItemsToTarget(req.TemplateID, req.TargetID)
	if err != nil {
		logger.Error("CopyAllChecklistTemplateItemsToTargetHandler: Error copying items from template %d to target %d: %v", req.TemplateID, req.TargetID, err)
		http.Error(w, "Failed to copy checklist items: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      fmt.Sprintf("Successfully copied %d items from template %d to target %d.", itemsCopied, req.TemplateID, req.TargetID),
		"items_copied": itemsCopied,
	})
	logger.Info("CopyAllChecklistTemplateItemsToTargetHandler: Copied %d items from template %d to target %d.", itemsCopied, req.TemplateID, req.TargetID)
}
