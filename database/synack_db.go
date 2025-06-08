package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"toolkit/logger"
	"toolkit/models"
)

func UpsertSynackTarget(targetData map[string]interface{}) (int64, error) {
	synackID, ok := targetData["id"].(string)
	if !ok || synackID == "" {
		return 0, fmt.Errorf("synack target data missing or invalid 'id' field (must be string)")
	}

	codename, _ := targetData["codename"].(string)
	organizationID, _ := targetData["organization_id"].(string)
	activatedAt, _ := targetData["activated_at"].(string)
	name, _ := targetData["name"].(string)
	category, _ := targetData["category"].(string)

	outageWindowsRaw, _ := json.Marshal(targetData["outage_windows"])
	vulnDiscoveryRaw, _ := json.Marshal(targetData["vulnerability_discovery"])
	collabCriteriaRaw, _ := json.Marshal(targetData["collaboration_criteria"])
	rawJSON, _ := json.Marshal(targetData)

	now := time.Now().UTC().Format(time.RFC3339)

	stmt, err := DB.Prepare(`
		INSERT INTO synack_targets (
			synack_target_id_str, codename, organization_id, activated_at, name, category,
			outage_windows_raw, vulnerability_discovery_raw, collaboration_criteria_raw, raw_json_details,
			first_seen_timestamp, last_seen_timestamp, status, is_active, deactivated_at, analytics_last_fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL)
		ON CONFLICT(synack_target_id_str) DO UPDATE SET
			codename = excluded.codename,
			organization_id = excluded.organization_id,
			activated_at = excluded.activated_at,
			name = excluded.name,
			category = excluded.category,
			outage_windows_raw = excluded.outage_windows_raw,
			vulnerability_discovery_raw = excluded.vulnerability_discovery_raw,
			collaboration_criteria_raw = excluded.collaboration_criteria_raw,
			raw_json_details = excluded.raw_json_details,
			last_seen_timestamp = excluded.last_seen_timestamp,
			is_active = TRUE,
			deactivated_at = NULL,
			analytics_last_fetched_at = synack_targets.analytics_last_fetched_at -- Preserve existing value on update
	`)
	if err != nil {
		logger.ProxyError("UpsertSynackTarget: Error preparing statement for Synack target ID '%s': %v", synackID, err)
		return 0, err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		synackID, codename, organizationID, activatedAt, name, category,
		string(outageWindowsRaw), string(vulnDiscoveryRaw), string(collabCriteriaRaw),
		string(rawJSON), now, now, "new", true,
	)

	if err != nil {
		logger.ProxyError("UpsertSynackTarget: Error executing statement for Synack target ID '%s': %v", synackID, err)
		return 0, err
	}

	var dbID int64
	err = DB.QueryRow("SELECT id FROM synack_targets WHERE synack_target_id_str = ?", synackID).Scan(&dbID)
	if err != nil {
		logger.ProxyError("UpsertSynackTarget: Error fetching db_id for Synack target ID '%s' after upsert: %v", synackID, err)
		return 0, fmt.Errorf("failed to fetch db_id after upsert for %s: %w", synackID, err)
	}
	logger.ProxyInfo("Successfully upserted Synack target: ID '%s', Codename '%s', DB_ID %d", synackID, codename, dbID)
	return dbID, nil
}

func DeactivateMissingSynackTargets(seenSynackIDs []string, currentTimestamp time.Time) error {
	if len(seenSynackIDs) == 0 {
		logger.ProxyInfo("DeactivateMissingSynackTargets: Received empty list of seen Synack IDs. No targets will be deactivated.")
		return nil
	}

	placeholders := strings.Repeat("?,", len(seenSynackIDs)-1) + "?"
	query := fmt.Sprintf(`
		UPDATE synack_targets
		SET is_active = FALSE, deactivated_at = ?
		WHERE is_active = TRUE AND synack_target_id_str NOT IN (%s)
	`, placeholders)

	args := make([]interface{}, 0, len(seenSynackIDs)+1)
	args = append(args, currentTimestamp.UTC().Format(time.RFC3339))
	for _, id := range seenSynackIDs {
		args = append(args, id)
	}

	result, err := DB.Exec(query, args...)
	if err != nil {
		logger.ProxyError("DeactivateMissingSynackTargets: Error executing update: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logger.ProxyInfo("DeactivateMissingSynackTargets: Marked %d targets as inactive.", rowsAffected)
	return nil
}

func GetSynackTargetAnalyticsFetchTime(synackTargetDBID int64) (*time.Time, error) {
	var fetchedAt sql.NullString
	err := DB.QueryRow("SELECT analytics_last_fetched_at FROM synack_targets WHERE id = ?", synackTargetDBID).Scan(&fetchedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no synack_target found with db_id %d", synackTargetDBID)
		}
		return nil, fmt.Errorf("querying analytics_last_fetched_at for target %d: %w", synackTargetDBID, err)
	}

	if !fetchedAt.Valid || fetchedAt.String == "" {
		return nil, nil
	}

	parsedTime, parseErr := time.Parse(time.RFC3339, fetchedAt.String)
	if parseErr != nil {
		parsedTime, parseErr = time.Parse("2006-01-02 15:04:05", fetchedAt.String) // Fallback for older format if any
		if parseErr != nil {
			return nil, fmt.Errorf("parsing analytics_last_fetched_at string '%s': %w", fetchedAt.String, parseErr)
		}
	}
	return &parsedTime, nil
}

func UpdateSynackTargetAnalyticsFetchTime(synackTargetDBID int64, fetchTime time.Time) error {
	stmt, err := DB.Prepare("UPDATE synack_targets SET analytics_last_fetched_at = ? WHERE id = ?")
	if err != nil {
		return fmt.Errorf("preparing update analytics_last_fetched_at statement for target %d: %w", synackTargetDBID, err)
	}
	defer stmt.Close()

	var valueToStore interface{}
	if fetchTime.IsZero() {
		valueToStore = nil
		logger.ProxyDebug("UpdateSynackTargetAnalyticsFetchTime: Storing NULL for analytics_last_fetched_at for target DB_ID %d", synackTargetDBID)
	} else {
		valueToStore = fetchTime.UTC().Format(time.RFC3339)
		logger.ProxyDebug("UpdateSynackTargetAnalyticsFetchTime: Storing %s for analytics_last_fetched_at for target DB_ID %d", valueToStore, synackTargetDBID)
	}

	_, err = stmt.Exec(valueToStore, synackTargetDBID)
	if err != nil {
		return fmt.Errorf("executing update analytics_last_fetched_at for target %d: %w", synackTargetDBID, err)
	}
	return nil
}

func StoreSynackTargetAnalytics(synackTargetDBID int64, analyticsCategories []models.SynackTargetAnalyticsCategory) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction to store analytics for target %d: %w", synackTargetDBID, err)
	}

	if _, err := tx.Exec("DELETE FROM synack_target_analytics_categories WHERE synack_target_db_id = ?", synackTargetDBID); err != nil {
		tx.Rollback()
		return fmt.Errorf("deleting old analytics for target %d: %w", synackTargetDBID, err)
	}

	stmt, err := tx.Prepare("INSERT INTO synack_target_analytics_categories (synack_target_db_id, category_name, count) VALUES (?, ?, ?) ON CONFLICT(synack_target_db_id, category_name) DO NOTHING")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("preparing insert statement for analytics categories (target %d): %w", synackTargetDBID, err)
	}
	defer stmt.Close()

	for _, cat := range analyticsCategories {
		if _, err := stmt.Exec(synackTargetDBID, cat.CategoryName, cat.Count); err != nil {
			tx.Rollback()
			return fmt.Errorf("inserting analytics category '%s' for target %d: %w", cat.CategoryName, synackTargetDBID, err)
		}
	}

	updateStmt, err := tx.Prepare("UPDATE synack_targets SET analytics_last_fetched_at = ? WHERE id = ?")
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("preparing update analytics_last_fetched_at statement within transaction for target %d: %w", synackTargetDBID, err)
	}
	defer updateStmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := updateStmt.Exec(now, synackTargetDBID); err != nil {
		tx.Rollback()
		return fmt.Errorf("executing update analytics_last_fetched_at within transaction for target %d: %w", synackTargetDBID, err)
	}

	logger.ProxyInfo("Successfully stored %d analytics categories for Synack target DB_ID %d and updated fetch time to %s", len(analyticsCategories), synackTargetDBID, now)
	return tx.Commit()
}

func GetSynackTargetAnalyticsPaginated(targetDbID int64, limit int, offset int, sortByColumn string, sortOrder string) ([]models.SynackTargetAnalyticsCategory, int64, error) {
	var analytics []models.SynackTargetAnalyticsCategory
	var totalRecords int64

	countQuery := `SELECT COUNT(*) FROM synack_target_analytics_categories WHERE synack_target_db_id = ?`
	err := DB.QueryRow(countQuery, targetDbID).Scan(&totalRecords)
	if err != nil {
		logger.Error("GetSynackTargetAnalyticsPaginated: Error counting categories for target_db_id %d: %v", targetDbID, err)
		return nil, 0, fmt.Errorf("counting categories for target %d: %w", targetDbID, err)
	}

	if totalRecords == 0 {
		return analytics, 0, nil
	}

	allowedSortColumns := map[string]bool{"category_name": true, "count": true}
	if !allowedSortColumns[sortByColumn] {
		sortByColumn = "category_name"
	}
	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		sortOrder = "ASC"
	}
	dbSortColumnSQL := sortByColumn
	if sortByColumn == "category_name" {
		dbSortColumnSQL += " COLLATE NOCASE"
	}

	query := fmt.Sprintf(`SELECT category_name, count
                          FROM synack_target_analytics_categories
                          WHERE synack_target_db_id = ?
                          ORDER BY %s %s
                          LIMIT ? OFFSET ?`, dbSortColumnSQL, sortOrder)

	rows, err := DB.Query(query, targetDbID, limit, offset)
	if err != nil {
		logger.Error("GetSynackTargetAnalyticsPaginated: Error querying findings analytics for target_db_id %d: %v", targetDbID, err)
		return nil, totalRecords, fmt.Errorf("querying findings analytics for target %d: %w", targetDbID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cat models.SynackTargetAnalyticsCategory
		cat.SynackTargetDBID = targetDbID
		if err := rows.Scan(&cat.CategoryName, &cat.Count); err != nil {
			logger.Error("GetSynackTargetAnalyticsPaginated: Error scanning analytics row for target_db_id %d: %v", targetDbID, err)
			return nil, totalRecords, fmt.Errorf("scanning analytics row for target %d: %w", targetDbID, err)
		}
		analytics = append(analytics, cat)
	}
	return analytics, totalRecords, rows.Err()
}

func ListAllSynackAnalyticsPaginated(limit int, offset int, sortByColumn string, sortOrder string) ([]models.SynackGlobalAnalyticsEntry, int64, error) {
	var entries []models.SynackGlobalAnalyticsEntry
	var totalRecords int64

	countQuery := `SELECT COUNT(*) FROM synack_target_analytics_categories`
	err := DB.QueryRow(countQuery).Scan(&totalRecords)
	if err != nil {
		logger.Error("ListAllSynackAnalyticsPaginated: Error counting all analytics: %v", err)
		return nil, 0, fmt.Errorf("counting all analytics: %w", err)
	}
	if totalRecords == 0 {
		return entries, 0, nil
	}

	allowedSortColumns := map[string]string{"target_codename": "st.codename", "category_name": "stac.category_name", "count": "stac.count"}
	dbSortColumnActual, ok := allowedSortColumns[sortByColumn]
	if !ok {
		dbSortColumnActual = "st.codename"
		sortByColumn = "target_codename" // For COLLATE logic
	}
	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		sortOrder = "ASC"
	}
	secondarySortColumn := "stac.id"
	if sortByColumn == "category_name" {
		secondarySortColumn = "st.codename"
	}
	dbSortColumnSQL := dbSortColumnActual
	secondarySortColumnSQL := secondarySortColumn
	if sortByColumn == "target_codename" || sortByColumn == "category_name" {
		dbSortColumnSQL += " COLLATE NOCASE"
	}
	if secondarySortColumn == "st.codename" || secondarySortColumn == "stac.category_name" {
		secondarySortColumnSQL += " COLLATE NOCASE"
	}

	query := fmt.Sprintf(`SELECT st.id AS target_db_id, st.codename AS target_codename,
	                             stac.category_name, stac.count
	                      FROM synack_target_analytics_categories stac
	                      JOIN synack_targets st ON stac.synack_target_db_id = st.id
	                      ORDER BY %s %s, %s %s
	                      LIMIT ? OFFSET ?`, dbSortColumnSQL, sortOrder, secondarySortColumnSQL, sortOrder)

	rows, err := DB.Query(query, limit, offset)
	if err != nil {
		logger.Error("ListAllSynackAnalyticsPaginated: Error querying all analytics: %v", err)
		return nil, totalRecords, fmt.Errorf("querying all analytics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry models.SynackGlobalAnalyticsEntry
		if err := rows.Scan(&entry.TargetDBID, &entry.TargetCodename, &entry.CategoryName, &entry.Count); err != nil {
			logger.Error("ListAllSynackAnalyticsPaginated: Error scanning analytics entry: %v", err)
			return nil, totalRecords, fmt.Errorf("scanning analytics entry: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, totalRecords, rows.Err()
}

func GetSynackTargetFindingsPaginated(targetDbID int64, limit int, offset int, sortByColumn string, sortOrder string) ([]models.SynackFinding, int64, error) {
	var findings []models.SynackFinding
	var totalRecords int64

	countQuery := `SELECT COUNT(*) FROM synack_findings WHERE synack_target_db_id = ?`
	err := DB.QueryRow(countQuery, targetDbID).Scan(&totalRecords)
	if err != nil {
		logger.Error("GetSynackTargetFindingsPaginated: Error counting findings for target_db_id %d: %v", targetDbID, err)
		return nil, 0, fmt.Errorf("counting findings for target %d: %w", targetDbID, err)
	}
	if totalRecords == 0 {
		return findings, 0, nil
	}

	allowedSortColumns := map[string]string{
		"synack_finding_id": "synack_finding_id", "title": "title", "category_name": "category_name",
		"severity": "severity", "status": "status", "amount_paid": "amount_paid",
		"vulnerability_url": "vulnerability_url", "reported_at": "reported_at", "closed_at": "closed_at", "id": "id",
	}
	dbSortColumn, ok := allowedSortColumns[sortByColumn]
	actualSortByColumnForCollate := sortByColumn
	if !ok {
		dbSortColumn = "reported_at"
		actualSortByColumnForCollate = "reported_at"
	}
	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		sortOrder = "DESC"
	}
	dbSortColumnSQL := dbSortColumn
	textSortColumns := map[string]bool{"synack_finding_id": true, "title": true, "category_name": true, "severity": true, "status": true, "vulnerability_url": true}
	if textSortColumns[actualSortByColumnForCollate] {
		dbSortColumnSQL += " COLLATE NOCASE"
	}
	secondarySortColumnSQL := "id"

	query := fmt.Sprintf(`SELECT id, synack_target_db_id, synack_finding_id, title, category_name, severity,
	                             status, amount_paid, vulnerability_url, reported_at, closed_at, raw_json_details
	                      FROM synack_findings
	                      WHERE synack_target_db_id = ?
	                      ORDER BY %s %s, %s %s
	                      LIMIT ? OFFSET ?`, dbSortColumnSQL, sortOrder, secondarySortColumnSQL, sortOrder)

	rows, err := DB.Query(query, targetDbID, limit, offset)
	if err != nil {
		logger.Error("GetSynackTargetFindingsPaginated: Error querying findings for target_db_id %d: %v", targetDbID, err)
		return nil, totalRecords, fmt.Errorf("querying findings for target %d: %w", targetDbID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var f models.SynackFinding
		var reportedAtScannable, closedAtScannable sql.NullTime
		var amountPaidScannable sql.NullFloat64
		err := rows.Scan(
			&f.DBID, &f.SynackTargetDBID, &f.SynackFindingID, &f.Title, &f.CategoryName,
			&f.Severity, &f.Status, &amountPaidScannable, &f.VulnerabilityURL,
			&reportedAtScannable, &closedAtScannable, &f.RawJSONDetails,
		)
		if err != nil {
			logger.Error("GetSynackTargetFindingsPaginated: Error scanning finding row for target_db_id %d: %v", targetDbID, err)
			return nil, totalRecords, fmt.Errorf("scanning finding row for target %d: %w", targetDbID, err)
		}
		if reportedAtScannable.Valid {
			f.ReportedAt = &reportedAtScannable.Time
		} else {
			f.ReportedAt = nil
		}
		if closedAtScannable.Valid {
			f.ClosedAt = &closedAtScannable.Time
		} else {
			f.ClosedAt = nil
		}
		if amountPaidScannable.Valid {
			f.AmountPaid = amountPaidScannable.Float64
		} else {
			f.AmountPaid = 0.0
		}
		findings = append(findings, f)
	}
	return findings, totalRecords, rows.Err()
}

func UpsertSynackFinding(finding models.SynackFinding) (int64, error) {
	stmt, err := DB.Prepare(`
		INSERT INTO synack_findings (
			synack_target_db_id, synack_finding_id, title, category_name, severity,
			status, amount_paid, vulnerability_url, reported_at, closed_at, raw_json_details,
			last_updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(synack_target_db_id, synack_finding_id) DO UPDATE SET
			title = excluded.title,
			category_name = excluded.category_name,
			severity = excluded.severity,
			status = excluded.status,
			amount_paid = excluded.amount_paid,
			vulnerability_url = excluded.vulnerability_url,
			reported_at = excluded.reported_at,
			closed_at = excluded.closed_at,
			raw_json_details = excluded.raw_json_details,
			last_updated_at = CURRENT_TIMESTAMP
		RETURNING id;`)
	if err != nil {
		logger.Error("UpsertSynackFinding: Error preparing statement for finding '%s' on target %d: %v", finding.SynackFindingID, finding.SynackTargetDBID, err)
		return 0, err
	}
	defer stmt.Close()

	var findingDBID int64
	err = stmt.QueryRow(
		finding.SynackTargetDBID, finding.SynackFindingID, finding.Title, finding.CategoryName, finding.Severity,
		finding.Status, finding.AmountPaid, finding.VulnerabilityURL, finding.ReportedAt, finding.ClosedAt, finding.RawJSONDetails,
	).Scan(&findingDBID)
	if err != nil {
		logger.Error("UpsertSynackFinding: Error executing statement for finding '%s' on target %d: %v", finding.SynackFindingID, finding.SynackTargetDBID, err)
		return 0, err
	}
	return findingDBID, nil
}

func ListSynackTargetsPaginated(limit int, offset int, sortByColumn string, sortOrder string, filterIsActive *bool) ([]models.SynackTarget, int64, error) {
	var targets []models.SynackTarget
	var totalRecords int64

	countQueryBase := "FROM synack_targets st"
	whereClause := ""
	args := []interface{}{}

	if filterIsActive != nil {
		whereClause = " WHERE st.is_active = ?"
		args = append(args, *filterIsActive)
	}

	countQuery := "SELECT COUNT(st.id) " + countQueryBase + whereClause
	err := DB.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		logger.Error("ListSynackTargetsPaginated: Error counting targets: %v", err)
		return nil, 0, fmt.Errorf("counting targets: %w", err)
	}
	if totalRecords == 0 {
		return targets, 0, nil
	}

	allowedSortColumns := map[string]string{
		"synack_target_id_str": "st.synack_target_id_str", "codename": "st.codename", "name": "st.name",
		"status": "st.status", "last_seen_timestamp": "st.last_seen_timestamp",
		"findings_count": "(SELECT COUNT(sf.id) FROM synack_findings sf WHERE sf.synack_target_db_id = st.id)", "id": "st.id",
	}
	dbSortColumn, ok := allowedSortColumns[sortByColumn]
	actualSortByColumnForCollate := sortByColumn
	if !ok {
		dbSortColumn = "st.last_seen_timestamp"
		actualSortByColumnForCollate = "last_seen_timestamp"
	}
	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		sortOrder = "DESC"
	}
	dbSortColumnSQL := dbSortColumn
	textSortColumns := map[string]bool{"synack_target_id_str": true, "codename": true, "name": true, "status": true}
	if textSortColumns[actualSortByColumnForCollate] {
		dbSortColumnSQL += " COLLATE NOCASE"
	}
	secondarySortColumnSQL := "st.id"

	query := fmt.Sprintf(`
		SELECT
			st.id, st.synack_target_id_str, st.codename, st.organization_id,
			st.activated_at, st.name, st.category, st.status,
			st.is_active, st.deactivated_at, st.notes,
			st.first_seen_timestamp, st.last_seen_timestamp, st.analytics_last_fetched_at,
			(SELECT COUNT(sf.id) FROM synack_findings sf WHERE sf.synack_target_db_id = st.id) as findings_count
		%s %s
		ORDER BY %s %s, %s %s
		LIMIT ? OFFSET ?`, countQueryBase, whereClause, dbSortColumnSQL, sortOrder, secondarySortColumnSQL, sortOrder)

	queryArgs := append(args, limit, offset)
	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		logger.Error("ListSynackTargetsPaginated: Error querying targets: %v", err)
		return nil, totalRecords, fmt.Errorf("querying targets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t models.SynackTarget
		var orgIDScannable, activatedAtScannable, nameScannable, categoryScannable, notesScannable sql.NullString
		var firstSeenScannable, lastSeenScannable, deactivatedScannable, analyticsFetchedScannable sql.NullTime
		err := rows.Scan(
			&t.DBID, &t.SynackTargetIDStr, &t.Codename, &orgIDScannable,
			&activatedAtScannable, &nameScannable, &categoryScannable, &t.Status,
			&t.IsActive, &deactivatedScannable, &notesScannable,
			&firstSeenScannable, &lastSeenScannable, &analyticsFetchedScannable,
			&t.FindingsCount,
		)
		if err != nil {
			logger.Error("ListSynackTargetsPaginated: Error scanning target row: %v", err)
			return nil, totalRecords, fmt.Errorf("scanning target row: %w", err)
		}
		t.OrganizationID = orgIDScannable.String
		t.ActivatedAt = activatedAtScannable.String
		t.Name = nameScannable.String
		t.Category = categoryScannable.String
		t.Notes = notesScannable.String
		if firstSeenScannable.Valid {
			t.FirstSeenTimestamp = firstSeenScannable.Time
		}
		if lastSeenScannable.Valid {
			t.LastSeenTimestamp = lastSeenScannable.Time
		}
		if deactivatedScannable.Valid {
			t.DeactivatedAt = &deactivatedScannable.Time
		}
		if analyticsFetchedScannable.Valid {
			t.AnalyticsLastFetchedAt = &analyticsFetchedScannable.Time
		}
		targets = append(targets, t)
	}
	return targets, totalRecords, rows.Err()
}
