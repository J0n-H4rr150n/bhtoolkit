package models

import (
	"database/sql"
	"time"
)

// ParameterizedURL represents a URL pattern identified in the application,
// often with example parameters and an associated log entry.
type ParameterizedURL struct {
	ID               int64          `json:"id"`
	TargetID         sql.NullInt64  `json:"target_id"` // Changed to sql.NullInt64
	RequestMethod    sql.NullString `json:"request_method"`
	RequestPath      sql.NullString `json:"request_path"`
	ParamKeys        string         `json:"param_keys"` // Comma-separated list of parameter keys
	ExampleFullURL   sql.NullString `json:"example_full_url,omitempty"`
	HTTPTrafficLogID sql.NullInt64  `json:"http_traffic_log_id,omitempty"` // Changed to sql.NullInt64
	Notes            sql.NullString `json:"notes,omitempty"`
	DiscoveredAt     time.Time      `json:"discovered_at"`
	LastSeenAt       time.Time      `json:"last_seen_at"`
	HitCount         int            `json:"hit_count,omitempty"`
}
