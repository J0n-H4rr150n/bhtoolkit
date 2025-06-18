package models

import (
	"database/sql"
	"time"
)

// NullString is a helper function to create a sql.NullString from a string.
// If the input string is empty, it returns a NullString with Valid set to false.
// Otherwise, it returns a NullString with the given string and Valid set to true.
func NullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{String: "", Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

// Platform, Target, ScopeRule, ScopeItemRequest, TargetCreateRequest remain the same

type Platform struct {
	ID   int64  `json:"id" example:"1" format:"int64" readOnly:"true"` // Unique identifier for the platform.
	Name string `json:"name" example:"HackerOne" binding:"required"`   // Name of the platform.
}

type ScopeItemRequest struct {
	Pattern     string `json:"pattern" example:"*.example.com" binding:"required"`                                    // The scope pattern (e.g., *.example.com, /api/v1/*).
	ItemType    string `json:"item_type,omitempty" example:"domain" enum:"domain,subdomain,ip_address,cidr,url_path"` // Type of scope item. Auto-detected if not provided.
	Description string `json:"description,omitempty" example:"Main application wildcard"`                             // Optional description for the scope item.
}

type TargetCreateRequest struct {
	PlatformID      int64              `json:"platform_id" example:"1" binding:"required"`                               // ID of the platform this target belongs to.
	Codename        string             `json:"codename" example:"Alpha Web App" binding:"required"`                      // A unique codename for the target within the platform.
	Link            string             `json:"link" example:"https://alpha.example.com" binding:"required" format:"url"` // Primary URL/link for the target.
	Notes           string             `json:"notes,omitempty" example:"Main web application for Alpha project."`        // Optional notes for the target.
	InScopeItems    []ScopeItemRequest `json:"in_scope_items,omitempty"`                                                 // List of items that are in scope.
	OutOfScopeItems []ScopeItemRequest `json:"out_of_scope_items,omitempty"`                                             // List of items that are out of scope.
}

type ScopeRule struct {
	ID          int64  `json:"id" example:"101" format:"int64" readOnly:"true"`
	TargetID    int64  `json:"target_id,omitempty" example:"1" format:"int64" readOnly:"true"`                                 // ID of the target this rule belongs to.
	ItemType    string `json:"item_type" example:"domain" enum:"domain,subdomain,ip_address,cidr,url_path" binding:"required"` // Type of scope item.
	Pattern     string `json:"pattern" example:"*.example.com" binding:"required"`                                             // The scope pattern.
	IsInScope   bool   `json:"is_in_scope" example:"true" binding:"required"`                                                  // True if the pattern is in scope, false if out of scope.
	IsWildcard  bool   `json:"is_wildcard" example:"true" readOnly:"true"`                                                     // True if the pattern contains a wildcard, auto-detected by backend.
	Description string `json:"description,omitempty" example:"Main application domain"`                                        // Optional description for the scope rule.
}

type Target struct {
	ID         int64       `json:"id" example:"1" format:"int64" readOnly:"true"`
	PlatformID int64       `json:"platform_id" example:"1" format:"int64"`
	Slug       string      `json:"slug,omitempty" example:"alpha-web-app" readOnly:"true"`
	Codename   string      `json:"codename" example:"Alpha Web App"`
	Link       string      `json:"link" example:"https://alpha.example.com" format:"url"`
	Notes      string      `json:"notes,omitempty" example:"Initial notes about the target."`
	ScopeRules []ScopeRule `json:"scope_rules,omitempty"` // Associated scope rules for the target (populated for GET by ID).
}

// TargetUpdateRequest defines the fields that can be updated for a target.
type TargetUpdateRequest struct {
	// Codename string `json:"codename"` // Optional: If you want to allow codename updates
	Link  string `json:"link"`
	Notes string `json:"notes"`
}

// UISettingsResponse defines the structure for UI specific settings returned by the API.
type UISettingsResponse struct {
	ShowSynackSection bool `json:"showSynackSection"`
}

// DiscoveredURL, HTTPTrafficLog, WebPage, APIEndpoint, PageAPIRelationship, AnalysisResult structs remain the same
type DiscoveredURL struct {
	ID               int64     `json:"id" readOnly:"true"`
	TargetID         *int64    `json:"target_id,omitempty"`
	HTTPTrafficLogID int64     `json:"http_traffic_log_id"`
	URL              string    `json:"url"`
	SourceType       string    `json:"source_type" example:"jsluice"`
	DiscoveredAt     time.Time `json:"discovered_at" readOnly:"true"`
}

type WebPage struct {
	ID           int64     `json:"id" readOnly:"true"`
	TargetID     int64     `json:"target_id"`
	URL          string    `json:"url" example:"https://example.com/dashboard"`
	Title        string    `json:"title,omitempty" example:"User Dashboard"`
	DiscoveredAt time.Time `json:"discovered_at" readOnly:"true"`
}

type APIEndpoint struct {
	ID             int64     `json:"id" readOnly:"true"`
	TargetID       int64     `json:"target_id"`
	Method         string    `json:"method" example:"GET"`
	PathPattern    string    `json:"path_pattern" example:"/api/users/{id}"`
	Description    string    `json:"description,omitempty" example:"Retrieves user details."`
	ParametersInfo string    `json:"parameters_info,omitempty" example:"{\"id\": \"integer\"}"`
	DiscoveredAt   time.Time `json:"discovered_at" readOnly:"true"`
}

type PageAPIRelationship struct {
	ID            int64  `json:"id" readOnly:"true"`
	WebPageID     int64  `json:"web_page_id"`
	APIEndpointID int64  `json:"api_endpoint_id"`
	TriggerInfo   string `json:"trigger_info,omitempty" example:"onClick_button_load_data"`
}

type AnalysisResult struct {
	ID           int64     `json:"id" readOnly:"true"`
	HTTPLogID    int64     `json:"http_log_id"`
	AnalysisType string    `json:"analysis_type" example:"js_links_jsluice"`
	ResultData   string    `json:"result_data" example:"{\"urls\":[\"example.com/path\"]}"`
	Timestamp    time.Time `json:"timestamp" readOnly:"true"`
}

// Note represents a user-created note.
type Note struct {
	ID        int64          `json:"id" readOnly:"true"`
	Title     sql.NullString `json:"title,omitempty"` // Optional title
	Content   string         `json:"content" binding:"required"`
	CreatedAt time.Time      `json:"created_at" readOnly:"true"`
	UpdatedAt time.Time      `json:"updated_at" readOnly:"true"`
}
