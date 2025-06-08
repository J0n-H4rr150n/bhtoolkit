package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"toolkit/models"
)

// CreatePlatform inserts a new platform into the database.
func CreatePlatform(name string) (models.Platform, error) {
	var p models.Platform
	p.Name = strings.TrimSpace(name)
	if p.Name == "" {
		return p, errors.New("platform name is required")
	}

	stmt, err := DB.Prepare("INSERT INTO platforms(name) VALUES(?)")
	if err != nil {
		return p, fmt.Errorf("preparing insert platform statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.Exec(p.Name)
	if err != nil {
		return p, fmt.Errorf("executing insert platform statement for name '%s': %w", p.Name, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return p, fmt.Errorf("getting last insert ID for platform '%s': %w", p.Name, err)
	}
	p.ID = id
	return p, nil
}

// GetAllPlatforms retrieves all platforms from the database.
func GetAllPlatforms() ([]models.Platform, error) {
	rows, err := DB.Query("SELECT id, name FROM platforms ORDER BY name ASC")
	if err != nil {
		return nil, fmt.Errorf("querying all platforms: %w", err)
	}
	defer rows.Close()

	var platforms []models.Platform
	for rows.Next() {
		var p models.Platform
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, fmt.Errorf("scanning platform row: %w", err)
		}
		platforms = append(platforms, p)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating platform rows: %w", err)
	}
	return platforms, nil
}

// GetPlatformByID retrieves a single platform by its ID.
func GetPlatformByID(platformID int64) (models.Platform, error) {
	var p models.Platform
	query := `SELECT id, name FROM platforms WHERE id = ?`
	err := DB.QueryRow(query, platformID).Scan(&p.ID, &p.Name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, fmt.Errorf("platform with ID %d not found", platformID)
		}
		return p, fmt.Errorf("querying platform ID %d: %w", platformID, err)
	}
	return p, nil
}

// UpdatePlatform updates the name of an existing platform.
func UpdatePlatform(platformID int64, name string) (models.Platform, error) {
	var p models.Platform
	p.Name = strings.TrimSpace(name)
	if p.Name == "" {
		return p, errors.New("platform name is required for update")
	}

	stmt, err := DB.Prepare("UPDATE platforms SET name = ? WHERE id = ?")
	if err != nil {
		return p, fmt.Errorf("preparing update platform statement for ID %d: %w", platformID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(p.Name, platformID)
	if err != nil {
		return p, fmt.Errorf("executing update platform statement for ID %d: %w", platformID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return p, fmt.Errorf("platform with ID %d not found for update", platformID)
	}

	p.ID = platformID // Name is already set
	return p, nil
}

// DeletePlatform deletes a platform by its ID.
func DeletePlatform(platformID int64) error {
	stmt, err := DB.Prepare("DELETE FROM platforms WHERE id = ?")
	if err != nil {
		return fmt.Errorf("preparing delete platform statement for ID %d: %w", platformID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(platformID)
	if err != nil {
		return fmt.Errorf("executing delete platform statement for ID %d: %w", platformID, err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("platform with ID %d not found for deletion", platformID)
	}
	return nil
}
