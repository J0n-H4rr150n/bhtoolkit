package database

import (
	"database/sql"
	"fmt"
	"toolkit/models"
)

// CreateTargetFinding inserts a new finding into the database.
func CreateTargetFinding(finding models.TargetFinding) (int64, error) {
	stmt, err := DB.Prepare(`
		INSERT INTO target_findings (
			target_id, http_traffic_log_id, title, description, payload,
			severity, status, cvss_score, cwe_id, finding_references, discovered_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return 0, fmt.Errorf("preparing create target finding statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(
		finding.TargetID, finding.HTTPTrafficLogID, finding.Title, finding.Description, finding.Payload,
		finding.Severity, finding.Status, finding.CVSSScore, finding.CWEID, finding.FindingReferences,
	)
	if err != nil {
		return 0, fmt.Errorf("executing create target finding statement: %w", err)
	}
	return result.LastInsertId()
}

// GetTargetFindingsByTargetID retrieves all findings for a given target_id.
func GetTargetFindingsByTargetID(targetID int64) ([]models.TargetFinding, error) {
	rows, err := DB.Query(`
		SELECT id, target_id, http_traffic_log_id, title, description, payload,
		       severity, status, cvss_score, cwe_id, finding_references, discovered_at, updated_at
		FROM target_findings
		WHERE target_id = ?
		ORDER BY updated_at DESC, id DESC
	`, targetID)
	if err != nil {
		return nil, fmt.Errorf("querying findings for target %d: %w", targetID, err)
	}
	defer rows.Close()

	var findings []models.TargetFinding
	for rows.Next() {
		var f models.TargetFinding
		if err := rows.Scan(
			&f.ID, &f.TargetID, &f.HTTPTrafficLogID, &f.Title, &f.Description, &f.Payload,
			&f.Severity, &f.Status, &f.CVSSScore, &f.CWEID, &f.FindingReferences, &f.DiscoveredAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning finding row for target %d: %w", targetID, err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// GetTargetFindingByID retrieves a single finding by its ID.
func GetTargetFindingByID(findingID int64) (models.TargetFinding, error) {
	var f models.TargetFinding
	err := DB.QueryRow(`
		SELECT id, target_id, http_traffic_log_id, title, description, payload,
		       severity, status, cvss_score, cwe_id, finding_references, discovered_at, updated_at
		FROM target_findings
		WHERE id = ?
	`, findingID).Scan(
		&f.ID, &f.TargetID, &f.HTTPTrafficLogID, &f.Title, &f.Description, &f.Payload,
		&f.Severity, &f.Status, &f.CVSSScore, &f.CWEID, &f.FindingReferences, &f.DiscoveredAt, &f.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return f, fmt.Errorf("finding with ID %d not found", findingID)
		}
		return f, fmt.Errorf("querying finding %d: %w", findingID, err)
	}
	return f, nil
}

// UpdateTargetFinding updates an existing finding.
func UpdateTargetFinding(finding models.TargetFinding) error {
	stmt, err := DB.Prepare(`
		UPDATE target_findings SET
			http_traffic_log_id = ?, title = ?, description = ?, payload = ?,
			severity = ?, status = ?, cvss_score = ?, cwe_id = ?, finding_references = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND target_id = ?
	`) // Ensure target_id matches for security/integrity
	if err != nil {
		return fmt.Errorf("preparing update target finding statement for finding %d: %w", finding.ID, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		finding.HTTPTrafficLogID, finding.Title, finding.Description, finding.Payload,
		finding.Severity, finding.Status, finding.CVSSScore, finding.CWEID, finding.FindingReferences,
		finding.ID, finding.TargetID,
	)
	return err
}

// DeleteTargetFinding deletes a finding by its ID.
func DeleteTargetFinding(findingID int64, targetID int64) error {
	// Optional: Add targetID to the query to ensure user can only delete findings for their current target context
	_, err := DB.Exec("DELETE FROM target_findings WHERE id = ? AND target_id = ?", findingID, targetID)
	return err
}
