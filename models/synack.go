package models

import (
	"time"
)

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
}

// SynackGlobalAnalyticsEntry represents an analytics category entry along with its parent target's details.
// Used for the global analytics view.
type SynackGlobalAnalyticsEntry struct {
	TargetDBID     int64  `json:"target_db_id"`
	TargetCodename string `json:"target_codename"`
	CategoryName   string `json:"category_name"`
	Count          int64  `json:"count"` // Changed to int64 to match COUNT(*) result
}

// SynackMission represents a mission entry in the database.
type SynackMission struct {
	ID                    int64      `json:"id" readOnly:"true"`
	SynackTaskID          string     `json:"synack_task_id" gorm:"unique;not null"`
	Title                 string     `json:"title,omitempty"`
	Description           string     `json:"description,omitempty"`
	CampaignName          string     `json:"campaign_name,omitempty"`
	OrganizationUID       string     `json:"organization_uid,omitempty"`
	ListingUID            string     `json:"listing_uid,omitempty"`
	ListingCodename       string     `json:"listing_codename,omitempty"`
	CampaignUID           string     `json:"campaign_uid,omitempty"`
	PayoutAmount          float64    `json:"payout_amount,omitempty"`
	PayoutCurrency        string     `json:"payout_currency,omitempty"`
	Status                string     `json:"status" gorm:"not null;default:'UNKNOWN'"`
	ClaimedByToolkitAt    *time.Time `json:"claimed_by_toolkit_at,omitempty"`
	SynackAPIClaimedAt    *time.Time `json:"synack_api_claimed_at,omitempty"`
	SynackAPIExpiresAt    *time.Time `json:"synack_api_expires_at,omitempty"`
	RawMissionDetailsJSON string     `json:"raw_mission_details_json,omitempty"`
	Notes                 string     `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"created_at" readOnly:"true"`
	UpdatedAt             time.Time  `json:"updated_at" readOnly:"true"`
}

// SynackAPIMission represents the structure of a mission object from the Synack API.
type SynackAPIMission struct {
	ID              string `json:"id"` // This is the Task ID
	Title           string `json:"title"`
	Description     string `json:"description"`
	CampaignName    string `json:"campaignName"`
	OrganizationUID string `json:"organizationUid"`
	ListingUID      string `json:"listingUid"`
	ListingCodename string `json:"listingCodename"`
	CampaignUID     string `json:"campaignUid"`
	Payout          struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
	} `json:"payout"`
	// Add other fields as necessary, e.g., status, expiresAt, etc.
}