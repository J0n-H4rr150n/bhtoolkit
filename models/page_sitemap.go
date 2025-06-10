package models

import (
	"database/sql"
	"time"
)

// Page represents a recorded page load, grouping related HTTP requests.
type Page struct {
	ID             int64          `json:"id"`
	TargetID       int64          `json:"target_id"`
	Name           string         `json:"name"`
	Description    sql.NullString `json:"description,omitempty"`
	StartTimestamp time.Time      `json:"start_timestamp"`
	EndTimestamp   sql.NullTime   `json:"end_timestamp,omitempty"` // Use sql.NullTime for nullable time
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DisplayOrder   int            `json:"display_order"`
}

// PageHTTPLog represents the association between a Page and an HTTPTrafficLog entry.
// This struct might not be directly exposed via API often, but is useful for DB operations.
type PageHTTPLog struct {
	PageID           int64 `json:"page_id"`
	HTTPTrafficLogID int64 `json:"http_traffic_log_id"`
	// You could add CreatedAt here if you want to track when the association was made,
	// though the primary key in the DB doesn't include it.
	// CreatedAt time.Time `json:"created_at"`
}
