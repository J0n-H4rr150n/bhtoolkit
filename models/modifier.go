package models

import (
	"database/sql"
)

// ModifierTask represents a task sent to the HTTP request modifier.
type ModifierTask struct {
	ID                 int64          `json:"id"`
	TargetID           sql.NullInt64  `json:"target_id,omitempty"` // Can be null if not tied to a specific target
	Name               string         `json:"name"`
	BaseRequestURL     string         `json:"base_request_url"`
	BaseRequestMethod  string         `json:"base_request_method"`
	BaseRequestHeaders sql.NullString `json:"base_request_headers,omitempty"` // Stored as JSON string
	BaseRequestBody    sql.NullString `json:"base_request_body,omitempty"`    // Stored as Base64 string
	DisplayOrder       int            `json:"display_order"`
	CreatedAt          sql.NullTime   `json:"created_at"`                     // Changed to sql.NullTime
	UpdatedAt          sql.NullTime   `json:"updated_at"`                     // Changed to sql.NullTime
	LastExecutedLogID  sql.NullInt64  `json:"last_executed_log_id,omitempty"` // Link to the last executed log entry
	SourceLogID        sql.NullInt64  `json:"source_log_id,omitempty"`
	SourceParamURLID   sql.NullInt64  `json:"source_param_url_id,omitempty"`
}

// AddModifierTaskRequest defines the payload for creating a new modifier task.
// It can be sourced from either an HTTP traffic log entry or a parameterized URL.
type AddModifierTaskRequest struct {
	HTTPTrafficLogID   int64  `json:"http_traffic_log_id,omitempty"`
	ParameterizedURLID int64  `json:"parameterized_url_id,omitempty"`
	NameOverride       string `json:"name_override,omitempty"` // Optional name to override default
	TargetID           int64  `json:"target_id,omitempty"`     // Optional target_id if not derivable from source
}
