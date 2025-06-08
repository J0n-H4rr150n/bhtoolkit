package models

import (
	"database/sql"
	"time"
)

// SitemapManualEntry represents an entry manually added to the sitemap
// from a specific HTTP traffic log item.
type SitemapManualEntry struct {
	ID               int64          `json:"id"`
	TargetID         int64          `json:"target_id"`
	HTTPTrafficLogID sql.NullInt64  `json:"http_traffic_log_id"` // Can be null if original log is deleted
	FolderPath       string         `json:"folder_path"`         // User-defined hierarchical path
	RequestMethod    string         `json:"request_method"`      // Copied from http_traffic_log
	RequestPath      string         `json:"request_path"`        // Path part of URL, copied from http_traffic_log
	Notes            sql.NullString `json:"notes,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// AddSitemapManualEntryRequest defines the expected payload for adding a manual sitemap entry.
type AddSitemapManualEntryRequest struct {
	HTTPTrafficLogID int64  `json:"http_log_id" binding:"required"`
	FolderPath       string `json:"folder_path" binding:"required"`
	Notes            string `json:"notes"`
}
