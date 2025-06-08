package database

import (
	"fmt"
	"toolkit/logger"
	"toolkit/models"
)

// CreateSitemapManualEntry inserts a new manual sitemap entry into the database.
func CreateSitemapManualEntry(entry models.SitemapManualEntry) (int64, error) {
	stmt, err := DB.Prepare(`INSERT INTO sitemap_manual_entries 
        (target_id, http_traffic_log_id, folder_path, request_method, request_path, notes) 
        VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		logger.Error("CreateSitemapManualEntry: Error preparing statement: %v", err)
		return 0, fmt.Errorf("preparing sitemap manual entry insert statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(entry.TargetID, entry.HTTPTrafficLogID, entry.FolderPath, entry.RequestMethod, entry.RequestPath, entry.Notes)
	if err != nil {
		logger.Error("CreateSitemapManualEntry: Error executing insert: %v", err)
		return 0, fmt.Errorf("executing sitemap manual entry insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Error("CreateSitemapManualEntry: Error getting last insert ID: %v", err)
		return 0, fmt.Errorf("getting last insert ID for sitemap manual entry: %w", err)
	}
	return id, nil
}

// GetSitemapManualEntriesByTargetID retrieves all manual sitemap entries for a given target ID.
func GetSitemapManualEntriesByTargetID(targetID int64) ([]models.SitemapManualEntry, error) {
	rows, err := DB.Query(`SELECT id, target_id, http_traffic_log_id, folder_path, request_method, request_path, notes, created_at, updated_at 
                           FROM sitemap_manual_entries WHERE target_id = ? ORDER BY folder_path ASC, request_path ASC`, targetID)
	if err != nil {
		return nil, fmt.Errorf("querying sitemap manual entries for target %d: %w", targetID, err)
	}
	defer rows.Close()

	var entries []models.SitemapManualEntry
	for rows.Next() {
		var entry models.SitemapManualEntry
		if err := rows.Scan(&entry.ID, &entry.TargetID, &entry.HTTPTrafficLogID, &entry.FolderPath, &entry.RequestMethod, &entry.RequestPath, &entry.Notes, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning sitemap manual entry for target %d: %w", targetID, err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
