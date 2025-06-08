package models

import (
	"database/sql"
	"time"
)

// ModifierTask represents an item sent to the modifier for manipulation.
type ModifierTask struct {
	ID                 int64         `json:"id"`
	TargetID           sql.NullInt64 `json:"target_id"` // If the original request was tied to a target
	ParameterizedURLID sql.NullInt64 `json:"parameterized_url_id"`
	OriginalLogID      sql.NullInt64 `json:"original_log_id"` // Could be from http_traffic_log
	Name               string        `json:"name"`            // User-defined or auto-generated name
	BaseRequestMethod  string        `json:"base_request_method"`
	BaseRequestURL     string        `json:"base_request_url"`
	BaseRequestHeaders string        `json:"base_request_headers"` // JSON string
	BaseRequestBody    string        `json:"base_request_body"`    // Base64 encoded
	Notes              string        `json:"notes"`
	DisplayOrder       int           `json:"display_order"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// ModifierTaskVersion represents a specific version of a request/response for a ModifierTask.
type ModifierTaskVersion struct {
	ID             int64 `json:"id"`
	ModifierTaskID int64 `json:"modifier_task_id"`
	// Fields for modified request and actual response will go here
	// e.g., ModifiedRequestURL, ModifiedHeaders, ModifiedBody, ResponseStatusCode, ResponseHeaders, ResponseBody
	CreatedAt time.Time `json:"created_at"`
}
