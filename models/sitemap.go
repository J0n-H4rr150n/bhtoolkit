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

// SitemapTreeNode represents a node in the sitemap tree view.
type SitemapTreeNode struct {
	Name            string             `json:"name"`                        // e.g., "api", "users", or full path for leaf endpoint
	FullPath        string             `json:"full_path"`                   // e.g., "/api", "/api/users"
	Children        []*SitemapTreeNode `json:"children,omitempty"`          // Child nodes (folders)
	Endpoints       []SitemapEndpoint  `json:"endpoints,omitempty"`         // Actual requests at this specific path
	IsManuallyAdded bool               `json:"is_manually_added,omitempty"` // True if this node represents a manually added folder/path
	ManualEntryID   int64              `json:"manual_entry_id,omitempty"`   // ID if this node is from a manual entry
	Notes           sql.NullString     `json:"notes,omitempty"`             // Notes if from a manual entry
}

// SitemapEndpoint represents an actual HTTP endpoint found at a particular path.
type SitemapEndpoint struct {
	HTTPTrafficLogID int64          `json:"http_traffic_log_id,omitempty"` // 0 if manually added without direct log link
	Method           string         `json:"method"`
	Path             string         `json:"path"` // The specific path of the endpoint
	StatusCode       sql.NullInt64  `json:"status_code,omitempty"`
	ResponseSize     sql.NullInt64  `json:"response_size,omitempty"`
	IsFavorite       sql.NullBool   `json:"is_favorite,omitempty"`
	IsManuallyAdded  bool           `json:"is_manually_added,omitempty"`
	ManualEntryID    int64          `json:"manual_entry_id,omitempty"`
	ManualEntryNotes sql.NullString `json:"manual_entry_notes,omitempty"`
}

// AddSitemapManualEntryRequest defines the expected payload for adding a manual sitemap entry.
type AddSitemapManualEntryRequest struct {
	HTTPTrafficLogID int64  `json:"http_log_id" binding:"required"`
	FolderPath       string `json:"folder_path" binding:"required"`
	Notes            string `json:"notes"`
}
