package models

import (
	"database/sql"
	"time"
)

// TargetFinding represents a security finding associated with a target.
type TargetFinding struct {
	ID                int64           `json:"id"`
	TargetID          int64           `json:"target_id"`
	HTTPTrafficLogID  sql.NullInt64   `json:"http_traffic_log_id"` // Can be null
	Title             string          `json:"title"`
	Description       sql.NullString  `json:"description"`
	Payload           sql.NullString  `json:"payload"`
	Severity          sql.NullString  `json:"severity"` // Informational, Low, Medium, High, Critical
	Status            string          `json:"status"`   // Open, Closed, Remediated, Accepted Risk
	CVSSScore         sql.NullFloat64 `json:"cvss_score"`
	CWEID             sql.NullInt64   `json:"cwe_id"`
	FindingReferences sql.NullString  `json:"finding_references"` // JSON string of URLs or IDs
	DiscoveredAt      time.Time       `json:"discovered_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}
