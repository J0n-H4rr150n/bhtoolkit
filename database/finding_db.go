package database

import (
	"database/sql"
	"fmt"
	"toolkit/logger"
	"toolkit/models"
)

// CreateTargetFinding inserts a new finding into the database.
func CreateTargetFinding(finding models.TargetFinding) (int64, error) {
	logger.Info("Creating Target Finding: %v", finding)
	stmt, err := DB.Prepare(`
		INSERT INTO target_findings (
			target_id, http_traffic_log_id, title, summary, description, steps_to_reproduce,
			impact, recommendations, payload, severity, status, cvss_score, cwe_id,
			finding_references, vulnerability_type_id, discovered_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		logger.Error("Error preparing create target finding statement: %v", err)
		return 0, fmt.Errorf("preparing create target finding statement: %w", err)
	}

	logger.Debug("Prepared statement for creating TargetFinding: %v", stmt)

	defer stmt.Close()

	result, err := stmt.Exec(
		finding.TargetID, finding.HTTPTrafficLogID, finding.Title, finding.Summary, finding.Description, finding.StepsToReproduce,
		finding.Impact, finding.Recommendations, finding.Payload, finding.Severity, finding.Status,
		finding.CVSSScore, finding.CWEID, finding.FindingReferences, finding.VulnerabilityTypeID,
	)
	if err != nil {
		return 0, fmt.Errorf("executing create target finding statement: %w", err)
	}
	return result.LastInsertId()

}

// GetTargetFindingsByTargetID retrieves all findings for a given target_id.
func GetTargetFindingsByTargetID(targetID int64) ([]models.TargetFinding, error) {
	logger.Info("Getting Target Finding by target id: %v", targetID)

	rows, err := DB.Query(`
		SELECT id, target_id, http_traffic_log_id, title, summary, description, steps_to_reproduce,
		       impact, recommendations, payload, severity, status, cvss_score, cwe_id,
		       finding_references, vulnerability_type_id, discovered_at, updated_at
		FROM target_findings
		WHERE target_id = ?
		ORDER BY updated_at DESC, id DESC
	`, targetID)

	logger.Debug("Prepared query for getting TargetFinding by target id %v: %v", targetID, rows)

	if err != nil {
		return nil, fmt.Errorf("querying findings for target %d: %w", targetID, err)
	}
	defer rows.Close()

	var findings []models.TargetFinding
	for rows.Next() {
		var f models.TargetFinding

		if err := rows.Scan(
			&f.ID, &f.TargetID, &f.HTTPTrafficLogID, &f.Title, &f.Summary, &f.Description, &f.StepsToReproduce,
			&f.Impact, &f.Recommendations, &f.Payload, &f.Severity, &f.Status, &f.CVSSScore, &f.CWEID, &f.FindingReferences, &f.VulnerabilityTypeID, &f.DiscoveredAt, &f.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning finding row for target %d: %w", targetID, err)
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()

}

// GetTargetFindingByID retrieves a single finding by its ID.
func GetTargetFindingByID(findingID int64) (models.TargetFinding, error) {
	logger.Info("Getting Target Finding by  finding id: %v", findingID)
	var f models.TargetFinding
	err := DB.QueryRow(`
		SELECT id, target_id, http_traffic_log_id, title, summary, description, steps_to_reproduce,
		       impact, recommendations, payload, severity, status, cvss_score, cwe_id,
		       finding_references, vulnerability_type_id, discovered_at, updated_at
		FROM target_findings

		WHERE id = ?
	`, findingID).Scan(
		&f.ID, &f.TargetID, &f.HTTPTrafficLogID, &f.Title, &f.Summary, &f.Description, &f.StepsToReproduce,
		&f.Impact, &f.Recommendations, &f.Payload, &f.Severity, &f.Status, &f.CVSSScore, &f.CWEID, &f.FindingReferences, &f.VulnerabilityTypeID, &f.DiscoveredAt, &f.UpdatedAt,
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
	logger.Info("Updating Target Finding: %v", finding)
	stmt, err := DB.Prepare(`
		UPDATE target_findings SET
			http_traffic_log_id = ?, title = ?, summary = ?, description = ?, steps_to_reproduce = ?, impact = ?, recommendations = ?, payload = ?,
			severity = ?, status = ?, cvss_score = ?, cwe_id = ?, finding_references = ?, vulnerability_type_id = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND target_id = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing update target finding statement for finding %d: %w", finding.ID, err)
	}
	defer stmt.Close()

	logger.Debug("Updated statement for updating TargetFinding: %v", stmt)
	_, err = stmt.Exec(
		finding.HTTPTrafficLogID, finding.Title, finding.Summary, finding.Description,
		finding.StepsToReproduce, finding.Impact, finding.Recommendations, finding.Payload,
		finding.Severity, finding.Status, finding.CVSSScore, finding.CWEID, finding.FindingReferences, finding.VulnerabilityTypeID,
		finding.ID, finding.TargetID,
	)
	return err
}

// DeleteTargetFinding deletes a finding by its ID.
// ADD LOGGING
func DeleteTargetFinding(findingID int64, targetID int64) error {
	logger.Info("Deleting Target Finding with finding id: %v, and target id: %v", findingID, targetID)
	_, err := DB.Exec("DELETE FROM target_findings WHERE id = ? AND target_id = ?", findingID, targetID)
	return err
}

// --- VulnerabilityType Functions ---

// CreateVulnerabilityType inserts a new vulnerability type into the database.
func CreateVulnerabilityType(vt models.VulnerabilityType) (int64, error) {
	logger.Info("Creating VulnerabilityType: %s", vt.Name)
	stmt, err := DB.Prepare(`
		INSERT INTO vulnerability_types (name, description, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		logger.Error("Error preparing create vulnerability type statement: %v", err)
		return 0, fmt.Errorf("preparing create vulnerability type statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(vt.Name, vt.Description)
	if err != nil {
		return 0, fmt.Errorf("executing create vulnerability type statement: %w", err)
	}
	return result.LastInsertId()
}

// GetAllVulnerabilityTypes retrieves all vulnerability types from the database.
func GetAllVulnerabilityTypes() ([]models.VulnerabilityType, error) {
	logger.Info("Getting all VulnerabilityTypes")
	rows, err := DB.Query(`
		SELECT id, name, description, created_at, updated_at
		FROM vulnerability_types
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("querying all vulnerability types: %w", err)
	}
	defer rows.Close()

	var types []models.VulnerabilityType
	for rows.Next() {
		var vt models.VulnerabilityType
		if err := rows.Scan(&vt.ID, &vt.Name, &vt.Description, &vt.CreatedAt, &vt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning vulnerability type row: %w", err)
		}
		types = append(types, vt)
	}
	return types, rows.Err()
}

// UpdateVulnerabilityType updates an existing vulnerability type.
func UpdateVulnerabilityType(vt models.VulnerabilityType) error {
	logger.Info("Updating VulnerabilityType ID: %d", vt.ID)
	stmt, err := DB.Prepare(`
		UPDATE vulnerability_types SET
			name = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing update vulnerability type statement for ID %d: %w", vt.ID, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(vt.Name, vt.Description, vt.ID)
	return err
}

// DeleteVulnerabilityType deletes a vulnerability type by its ID.
// Note: This will fail if there are target_findings referencing this type,
// unless ON DELETE SET NULL or ON DELETE CASCADE is set on the foreign key.
// The current migration does not set this, so deletion might be restricted.
func DeleteVulnerabilityType(id int64) error {
	logger.Info("Deleting VulnerabilityType ID: %d", id)
	// Before deleting, consider implications if findings are using this type.
	// You might want to check if any findings reference this ID and prevent deletion,
	// or handle it (e.g., set their vulnerability_type_id to NULL).
	// For now, a direct delete is attempted.
	_, err := DB.Exec("DELETE FROM vulnerability_types WHERE id = ?", id)
	return err
}

// GetVulnerabilityTypeByID retrieves a single vulnerability type by its ID.
func GetVulnerabilityTypeByID(id int64) (models.VulnerabilityType, error) {
	logger.Info("Getting VulnerabilityType ID: %d", id)
	var vt models.VulnerabilityType
	err := DB.QueryRow(`
		SELECT id, name, description, created_at, updated_at
		FROM vulnerability_types
		WHERE id = ?
	`, id).Scan(&vt.ID, &vt.Name, &vt.Description, &vt.CreatedAt, &vt.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return vt, fmt.Errorf("vulnerability type with ID %d not found", id)
		}
		logger.Error("Error querying vulnerability type ID %d: %v", id, err)
		return vt, fmt.Errorf("querying vulnerability type ID %d: %w", id, err)
	}
	return vt, nil
}
