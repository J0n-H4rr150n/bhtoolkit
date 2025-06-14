package models

import (
	"database/sql"
	"time"
)

// ChecklistTemplate represents a reusable checklist template.
type ChecklistTemplate struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Description sql.NullString `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ChecklistTemplateItem represents an item within a ChecklistTemplate.
type ChecklistTemplateItem struct {
	ID              int64          `json:"id"`
	TemplateID      int64          `json:"template_id"`
	ItemText        string         `json:"item_text"`
	ItemCommandText sql.NullString `json:"item_command_text,omitempty"`
	Notes           sql.NullString `json:"notes,omitempty"`
	DisplayOrder    int            `json:"display_order"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// TargetChecklistItem represents a checklist item applied to a specific target.
type TargetChecklistItem struct {
	ID              int64          `json:"id"`
	TargetID        int64          `json:"target_id"`
	ItemText        string         `json:"item_text"`
	ItemCommandText sql.NullString `json:"item_command_text,omitempty"`
	Notes           sql.NullString `json:"notes,omitempty"`
	IsCompleted     bool           `json:"is_completed"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// PaginatedChecklistTemplateItemsResponse is the structure for paginated checklist template item responses.
type PaginatedChecklistTemplateItemsResponse struct {
	Page         int                     `json:"page"`
	Limit        int                     `json:"limit"`
	TotalRecords int64                   `json:"total_records"`
	TotalPages   int64                   `json:"total_pages"` // Changed to int64 to match calculation
	Items        []ChecklistTemplateItem `json:"items"`
}

// PaginatedTargetChecklistItemsResponse is the structure for paginated target checklist item responses.
type PaginatedTargetChecklistItemsResponse struct {
	Page                           int                   `json:"page"`
	Limit                          int                   `json:"limit"`
	TotalRecords                   int64                 `json:"total_records"`
	TotalPages                     int                   `json:"total_pages"`
	TotalCompletedRecordsForFilter int64                 `json:"total_completed_records_for_filter"` // New field
	Items                          []TargetChecklistItem `json:"items"`
	SortBy                         string                `json:"sort_by,omitempty"`
	SortOrder                      string                `json:"sort_order,omitempty"`
	Filter                         string                `json:"filter,omitempty"`
	ShowIncompleteOnly             bool                  `json:"show_incomplete_only,omitempty"`
}
