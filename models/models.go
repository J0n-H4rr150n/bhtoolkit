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

type SynackTarget struct {
	DBID                      int64      `json:"db_id,omitempty" readOnly:"true"`
	SynackTargetIDStr         string     `json:"synack_target_id_str" example:"abcdef12345"`
	Codename                  string     `json:"codename,omitempty" example:"ProjectUnicorn"`
	OrganizationID            string     `json:"organization_id,omitempty" example:"org_xyz"`
	ActivatedAt               string     `json:"activated_at,omitempty" example:"2023-01-15T10:00:00Z"`
	Name                      string     `json:"name,omitempty" example:"Unicorn Web Application"`
	Category                  string     `json:"category,omitempty" example:"Web Application"`
	OutageWindowsRaw          string     `json:"outage_windows_raw,omitempty"`
	VulnerabilityDiscoveryRaw string     `json:"vulnerability_discovery_raw,omitempty"`
	CollaborationCriteriaRaw  string     `json:"collaboration_criteria_raw,omitempty"`
	RawJSONDetails            string     `json:"raw_json_details,omitempty"`
	FirstSeenTimestamp        time.Time  `json:"first_seen_timestamp,omitempty" readOnly:"true"`
	LastSeenTimestamp         time.Time  `json:"last_seen_timestamp,omitempty" readOnly:"true"`
	Status                    string     `json:"status,omitempty" example:"new"`
	IsActive                  bool       `json:"is_active" example:"true"`
	DeactivatedAt             *time.Time `json:"deactivated_at,omitempty" readOnly:"true" swaggertype:"string" format:"date-time"`
	Notes                     string     `json:"notes,omitempty"`
	AnalyticsLastFetchedAt    *time.Time `json:"analytics_last_fetched_at,omitempty" readOnly:"true" swaggertype:"string" format:"date-time"` // Added field
	FindingsCount             int64      `json:"findings_count"`
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

// PromoteSynackTargetRequest defines the request body for promoting a Synack target.
type PromoteSynackTargetRequest struct {
	SynackTargetIDStr       string             `json:"synack_target_id_str" binding:"required"`
	PlatformID              int64              `json:"platform_id" binding:"required"` // ID of the main platform to assign to
	CodenameOverride        string             `json:"codename_override,omitempty"`
	LinkOverride            string             `json:"link_override,omitempty"` // Strongly recommended
	NotesOverride           string             `json:"notes_override,omitempty"`
	InScopeItemsOverride    []ScopeItemRequest `json:"in_scope_items_override,omitempty"`
	OutOfScopeItemsOverride []ScopeItemRequest `json:"out_of_scope_items_override,omitempty"`
}

// SynackTargetAnalyticsCategory represents a single category's analytics data from Synack.
type SynackTargetAnalyticsCategory struct {
	SynackTargetDBID int64  `json:"-"` // Internal DB ID of the parent SynackTarget
	CategoryName     string `json:"category_name" example:"Cross-Site Scripting (XSS)"`
	Count            int64  `json:"count" example:"5"` // Changed to int64 to match COUNT(*) result
}

// SynackFinding represents an individual vulnerability finding from Synack.
type SynackFinding struct {
	DBID             int64      `json:"db_id,omitempty" readOnly:"true"`      // Internal database ID
	SynackTargetDBID int64      `json:"synack_target_db_id"`                  // FK to synack_targets table
	SynackFindingID  string     `json:"synack_finding_id" example:"SF-12345"` // Unique ID from Synack for the finding
	Title            string     `json:"title" example:"Reflected XSS on /search"`
	CategoryName     string     `json:"category_name" example:"Cross-Site Scripting (XSS)"` // Vulnerability category
	Severity         string     `json:"severity,omitempty" example:"High"`                  // e.g., Critical, High, Medium, Low
	Status           string     `json:"status,omitempty" example:"Accepted"`                // e.g., Reported, Accepted, Resolved, Paid
	AmountPaid       float64    `json:"amount_paid,omitempty" example:"750.00"`             // Amount paid for this specific finding
	VulnerabilityURL string     `json:"vulnerability_url,omitempty" example:"https://target.com/search?q=<script>"`
	ReportedAt       *time.Time `json:"reported_at,omitempty" swaggertype:"string" format:"date-time"` // Timestamp when reported
	ClosedAt         *time.Time `json:"closed_at,omitempty" swaggertype:"string" format:"date-time"`   // Timestamp when closed/resolved
	RawJSONDetails   string     `json:"raw_json_details,omitempty"`                                    // Full JSON details of the finding from Synack
	// LastUpdatedAt is managed by DB or application logic on change, not directly part of Synack's core finding data model
}

// SynackGlobalAnalyticsEntry represents an analytics category entry along with its parent target's details.
// Used for the global analytics view.
type SynackGlobalAnalyticsEntry struct {
	TargetDBID     int64  `json:"target_db_id"`
	TargetCodename string `json:"target_codename"`
	CategoryName   string `json:"category_name"`
	Count          int64  `json:"count"` // Changed to int64 to match COUNT(*) result
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

type HTTPTrafficLog struct {
	ID                   int64     `json:"id" readOnly:"true"`
	TargetID             *int64    `json:"target_id,omitempty"`
	Timestamp            time.Time `json:"timestamp" readOnly:"true"`
	RequestMethod        string    `json:"request_method" example:"GET"`
	RequestURL           string    `json:"request_url" example:"https://example.com/api/data?id=123"`
	RequestHTTPVersion   string    `json:"request_http_version" example:"HTTP/1.1"`
	RequestHeaders       string    `json:"request_headers" example:"{\"Content-Type\":[\"application/json\"]}"`
	RequestBody          []byte    `json:"request_body,omitempty"`
	ResponseStatusCode   int       `json:"response_status_code,omitempty" example:"200"`
	ResponseReasonPhrase string    `json:"response_reason_phrase,omitempty" example:"OK"`
	ResponseHTTPVersion  string    `json:"response_http_version,omitempty" example:"HTTP/1.1"`
	ResponseHeaders      string    `json:"response_headers,omitempty" example:"{\"Content-Type\":[\"application/json\"]}"`
	ResponseBody         []byte    `json:"response_body,omitempty"`
	ResponseContentType  string    `json:"response_content_type,omitempty" example:"application/json"`
	ResponseBodySize     int64     `json:"response_body_size,omitempty" example:"1024"`
	DurationMs           int64     `json:"duration_ms,omitempty" example:"150"`
	ClientIP             string    `json:"client_ip,omitempty" example:"192.168.1.100"`
	ServerIP             string    `json:"server_ip,omitempty" example:"203.0.113.45"`
	IsHTTPS              bool      `json:"is_https" example:"true"`
	IsPageCandidate      bool      `json:"is_page_candidate" example:"false"`
	Notes                string    `json:"notes,omitempty"`
	IsFavorite           bool      `json:"is_favorite"` // Added for favorite logs
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

type TargetChecklistItem struct {
	ID              int64          `json:"id"`
	TargetID        int64          `json:"target_id"`
	ItemText        string         `json:"item_text"`
	ItemCommandText sql.NullString `json:"item_command_text,omitempty"` // New field for the command
	Notes           sql.NullString `json:"notes"`                       // Use sql.NullString for nullable TEXT
	IsCompleted     bool           `json:"is_completed"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// ChecklistTemplate represents a predefined checklist.
type ChecklistTemplate struct {
	ID          int64          `json:"id" example:"1" format:"int64" readOnly:"true"`
	Name        string         `json:"name" example:"Web Application - Basic Scan" binding:"required"`
	Description sql.NullString `json:"description,omitempty" example:"A basic checklist for web application security."`
	// Items       []ChecklistTemplateItem `json:"items,omitempty"` // Might be populated on specific request
}

// ChecklistTemplateItem represents an item within a ChecklistTemplate.
type ChecklistTemplateItem struct {
	ID              int64          `json:"id" example:"101" format:"int64" readOnly:"true"`
	TemplateID      int64          `json:"template_id" example:"1" format:"int64"` // FK to ChecklistTemplate
	ItemText        string         `json:"item_text" example:"Check for SQL Injection" binding:"required"`
	ItemCommandText sql.NullString `json:"item_command_text,omitempty" example:"sqlmap -u '...' --batch --dbs"` // New field for the command
	Notes           sql.NullString `json:"notes,omitempty" example:"Focus on input fields and URL parameters."`
	DisplayOrder    int            `json:"display_order,omitempty" example:"1"` // Optional: for ordering items within a template
}

// PaginatedChecklistTemplateItemsResponse is the structure for paginated checklist template items API response.
type PaginatedChecklistTemplateItemsResponse struct {
	Page         int                     `json:"page"`
	Limit        int                     `json:"limit"`
	TotalRecords int64                   `json:"total_records"`
	TotalPages   int64                   `json:"total_pages"`
	Items        []ChecklistTemplateItem `json:"items"`
}

// Note represents a user-created note.
type Note struct {
	ID        int64          `json:"id" readOnly:"true"`
	Title     sql.NullString `json:"title,omitempty"` // Optional title
	Content   string         `json:"content" binding:"required"`
	CreatedAt time.Time      `json:"created_at" readOnly:"true"`
	UpdatedAt time.Time      `json:"updated_at" readOnly:"true"`
}
