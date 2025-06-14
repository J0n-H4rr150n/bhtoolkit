package models

import (
	"database/sql"
	"time"
)

// ModifierTask represents a task stored for the HTTP request modifier.
type ModifierTask struct {
	ID                       int64          `json:"id"`
	TargetID                 sql.NullInt64  `json:"target_id,omitempty"`
	Name                     string         `json:"name"`
	BaseRequestMethod        string         `json:"base_request_method"`
	BaseRequestURL           string         `json:"base_request_url"`
	BaseRequestHeaders       sql.NullString `json:"base_request_headers,omitempty"` // JSON string of headers
	BaseRequestBody          sql.NullString `json:"base_request_body,omitempty"`    // Base64 encoded body string
	LastExecutedLogID        sql.NullInt64  `json:"last_executed_log_id,omitempty"`
	SourceLogID              sql.NullInt64  `json:"source_log_id,omitempty"`       // Original http_traffic_log.id
	SourceParameterizedURLID sql.NullInt64  `json:"source_param_url_id,omitempty"` // Original parameterized_urls.id
	DisplayOrder             int            `json:"display_order"`                 // For ordering in the UI
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
}

// AddModifierTaskRequest defines the payload for creating a new modifier task
// based on an existing log entry or a parameterized URL.
type AddModifierTaskRequest struct {
	HTTPTrafficLogID   int64 `json:"http_traffic_log_id,omitempty"`
	ParameterizedURLID int64 `json:"parameterized_url_id,omitempty"`
	// TargetID might be implicitly derived from the source or explicitly set.
	// Name could be auto-generated or provided.
}
