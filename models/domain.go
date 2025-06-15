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
}
