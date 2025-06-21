package models

import (
	"database/sql"
	"time"
)

// TargetFinding represents a security finding associated with a target.
type TargetFinding struct {
	ID                  int64           `json:"id"`
	TargetID            int64           `json:"target_id"`
	HTTPTrafficLogID    sql.NullInt64   `json:"http_traffic_log_id,omitempty"` // Can be null
	Title               string          `json:"title"`
	Summary             sql.NullString  `json:"summary,omitempty"` // New field
	Description         sql.NullString  `json:"description,omitempty"`
	StepsToReproduce    sql.NullString  `json:"steps_to_reproduce,omitempty"` // New field
	Impact              sql.NullString  `json:"impact,omitempty"`             // New field
	Recommendations     sql.NullString  `json:"recommendations,omitempty"`    // New field
	Payload             sql.NullString  `json:"payload,omitempty"`
	Severity            sql.NullString  `json:"severity,omitempty"` // Informational, Low, Medium, High, Critical
	Status              string          `json:"status"`             // Open, Closed, Remediated, Accepted Risk
	CVSSScore           sql.NullFloat64 `json:"cvss_score,omitempty"`
	CWEID               sql.NullInt64   `json:"cwe_id,omitempty"`
	FindingReferences   sql.NullString  `json:"finding_references,omitempty"`    // JSON string of URLs or IDs
	VulnerabilityTypeID sql.NullInt64   `json:"vulnerability_type_id,omitempty"` // New field, FK to vulnerability_types
	DiscoveredAt        time.Time       `json:"discovered_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// FindingLink is a lightweight struct for linking findings.
type FindingLink struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}
