package models

import (
	"database/sql"
	"time"
)

// ParameterizedURL represents a unique combination of a URL path and its parameter keys.
type ParameterizedURL struct {
	ID               int64          `json:"id"`
	TargetID         int64          `json:"target_id"`
	HTTPTrafficLogID int64          `json:"http_traffic_log_id"`
	RequestMethod    string         `json:"request_method"`
	RequestPath      string         `json:"request_path"`
	ParamKeys        string         `json:"param_keys"` // Sorted, comma-separated
	ExampleFullURL   string         `json:"example_full_url"`
	Notes            sql.NullString `json:"notes"`
	DiscoveredAt     time.Time      `json:"discovered_at"`
	LastSeenAt       time.Time      `json:"last_seen_at"`
	HitCount         int            `json:"hit_count,omitempty"` // For aggregated views, not directly in table
}
