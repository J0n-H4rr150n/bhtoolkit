package models

import (
	"database/sql"
	"time"
)

// Domain represents a discovered domain or subdomain.
type Domain struct {
	ID              int64          `json:"id"`
	TargetID        int64          `json:"target_id"` // Foreign key to targets table
	DomainName      string         `json:"domain_name"`
	Source          sql.NullString `json:"source,omitempty"` // How it was discovered (e.g., 'subfinder', 'manual', 'proxy', 'scope_import')
	IsInScope       bool           `json:"is_in_scope"`      // Is this domain explicitly in scope?
	Notes           sql.NullString `json:"notes,omitempty"`  // User notes, or original wildcard pattern if from scope_import
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	IsFavorite      bool           `json:"is_favorite"`       // New field for favorite status
	IsWildcardScope bool           `json:"is_wildcard_scope"` // True if this entry was derived from a wildcard scope rule (e.g., *.example.com)

	// Fields for httpx results
	HTTPStatusCode    sql.NullInt64  `json:"http_status_code,omitempty"`
	HTTPContentLength sql.NullInt64  `json:"http_content_length,omitempty"`
	HTTPTitle         sql.NullString `json:"http_title,omitempty"`
	HTTPServer        sql.NullString `json:"http_server,omitempty"`
	HTTPTech          sql.NullString `json:"http_tech,omitempty"`    // Comma-separated list of technologies
	HttpxFullJson     sql.NullString `json:"httpx_full_json,omitempty"` // Store the full JSON output from httpx
}

// PaginatedDomainsResponse is the structure for paginated domain results.
type PaginatedDomainsResponse struct {
	Page         int      `json:"page"`
	Limit        int      `json:"limit"`
	TotalRecords int64    `json:"total_records"`
	TotalPages   int64    `json:"total_pages"`
	SortBy       string   `json:"sort_by,omitempty"`
	SortOrder    string   `json:"sort_order,omitempty"`
	Filter       string   `json:"filter,omitempty"` // General filter text, if applicable
	// Distinct values for dropdown filters
	DistinctHttpStatusCodes []sql.NullInt64  `json:"distinct_http_status_codes,omitempty"`
	DistinctHttpServers     []sql.NullString `json:"distinct_http_servers,omitempty"`
	DistinctHttpTechs       []sql.NullString `json:"distinct_http_techs,omitempty"`
	Records      []Domain `json:"records"`
}

// DomainFilters defines the available filters for querying domains.
type DomainFilters struct {
	TargetID         int64
	Page             int
	Limit            int
	SortBy           string
	SortOrder        string
	DomainNameSearch string // Filter by domain name (e.g., using LIKE)
	SourceSearch     string // Filter by source (e.g., using LIKE)
	IsInScope        *bool  // Pointer to allow filtering by true, false, or not filtering at all
	IsFavorite       *bool  `json:"is_favorite,omitempty"` // New filter for favorite status
	HttpxScanStatus  string `json:"httpx_scan_status,omitempty"`  // Filter by httpx scan status: "all", "scanned", "not_scanned"
	FilterHTTPStatusCode string `json:"filter_http_status_code,omitempty"` // New: For exact status code match
	FilterHTTPServer     string `json:"filter_http_server,omitempty"`      // New: For exact server match
	FilterHTTPTech       string `json:"filter_http_tech,omitempty"`        // New: For exact tech string match
}
