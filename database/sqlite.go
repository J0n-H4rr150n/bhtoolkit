package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"toolkit/logger"
	"toolkit/models"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dataSourceName string) error {
	var err error
	dbDir := filepath.Dir(dataSourceName)
	if dbDir != "." && dbDir != "" {
		if err := os.MkdirAll(dbDir, 0750); err != nil {
			logger.Error("Failed to create database directory %s: %v", dbDir, err)
			return fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
		}
	}

	DB, err = sql.Open("sqlite3", dataSourceName+"?_foreign_keys=on")
	if err != nil {
		logger.Error("Failed to open database: %v", err)
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err = DB.Ping(); err != nil {
		logger.Error("Failed to connect to database: %v", err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	migrationsPath := "file://database/migrations"
	m, err := migrate.New(
		migrationsPath,
		fmt.Sprintf("sqlite3://%s", dataSourceName+"?_foreign_keys=on"),
	)
	if err != nil {
		logger.Error("Failed to initialize migrations: %v (path: %s)", err, migrationsPath)
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}

	logger.Info("Applying database migrations...")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		logger.Error("Failed to apply migrations: %v", err)
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	logger.Info("Database migrations applied successfully (or no changes).")
	return seedInitialChecklistTemplates()
}

func columnExists(tableName string, columnName string) (bool, error) {
	rows, err := DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typeStr string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk); err == nil {
			if name == columnName {
				return true, nil
			}
		}
	}
	return false, rows.Err()
}

func GetAllChecklistTemplates() ([]models.ChecklistTemplate, error) {
	rows, err := DB.Query("SELECT id, name, description FROM checklist_templates ORDER BY name ASC")
	if err != nil {
		return nil, fmt.Errorf("querying checklist templates: %w", err)
	}
	defer rows.Close()

	var templates []models.ChecklistTemplate
	for rows.Next() {
		var t models.ChecklistTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Description); err != nil {
			return nil, fmt.Errorf("scanning checklist template row: %w", err)
		}
		templates = append(templates, t)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating checklist template rows: %w", err)
	}
	return templates, nil
}

func GetChecklistTemplateItemsPaginated(templateID int64, limit int, offset int) ([]models.ChecklistTemplateItem, int64, error) {
	var items []models.ChecklistTemplateItem
	var totalRecords int64

	countQuery := "SELECT COUNT(*) FROM checklist_template_items WHERE template_id = ?"
	err := DB.QueryRow(countQuery, templateID).Scan(&totalRecords)
	if err != nil {
		return nil, 0, fmt.Errorf("counting items for template %d: %w", templateID, err)
	}

	if totalRecords == 0 {
		return items, 0, nil
	}

	query := `SELECT id, template_id, item_text, item_command_text, notes, display_order
              FROM checklist_template_items
              WHERE template_id = ?
              ORDER BY display_order ASC, id ASC
              LIMIT ? OFFSET ?`

	rows, err := DB.Query(query, templateID, limit, offset)
	if err != nil {
		return nil, totalRecords, fmt.Errorf("querying items for template %d: %w", templateID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.ChecklistTemplateItem
		if err := rows.Scan(&item.ID, &item.TemplateID, &item.ItemText, &item.ItemCommandText, &item.Notes, &item.DisplayOrder); err != nil {
			return nil, totalRecords, fmt.Errorf("scanning item row for template %d: %w", templateID, err)
		}
		items = append(items, item)
	}
	return items, totalRecords, rows.Err()
}

func seedInitialChecklistTemplates() error {
	templates := []models.ChecklistTemplate{
		{Name: "Basic Web Application Scan", Description: sql.NullString{String: "A fundamental checklist for initial web application assessments.", Valid: true}},
		{Name: "API Security Top 10 (OWASP)", Description: sql.NullString{String: "Checklist based on OWASP API Security Top 10 vulnerabilities.", Valid: true}},
		{Name: "Tool Commands", Description: sql.NullString{String: "Commands for various tools.", Valid: true}},
		{Name: "Tool Commands with Proxy", Description: sql.NullString{String: "Commands for various tools with proxy support.", Valid: true}},
	}

	itemsMap := map[string][]struct {
		ItemText        string
		ItemCommandText string
		Notes           string
		DisplayOrder    int
	}{
		"Basic Web Application Scan": {
			{ItemText: "Identify application entry points (URLs, forms, parameters).", DisplayOrder: 1, Notes: "Map out the application surface."},
			{ItemText: "Check for common misconfigurations (default credentials, exposed admin panels).", DisplayOrder: 2, Notes: "Use tools like Nikto, Dirb."},
			{ItemText: "Test for basic input validation (XSS, SQLi placeholders).", DisplayOrder: 3, Notes: "Manually test and use scanners."},
			{ItemText: "Review session management mechanisms.", DisplayOrder: 4, Notes: "Check cookie flags, session fixation."},
			{ItemText: "Check for information leakage in HTTP headers and responses.", DisplayOrder: 5, Notes: "Server versions, verbose errors."},
		},
		"API Security Top 10 (OWASP)": {
			{ItemText: "API1:2023 - Broken Object Level Authorization", DisplayOrder: 1, Notes: "Test accessing resources of other users."},
			{ItemText: "API2:2023 - Broken Authentication", DisplayOrder: 2, Notes: "Check for weak credentials, JWT flaws."},
			{ItemText: "API3:2023 - Broken Object Property Level Authorization", DisplayOrder: 3, Notes: "Verify access controls on individual object properties."},
			{ItemText: "API4:2023 - Unrestricted Resource Consumption", DisplayOrder: 4, Notes: "Test for rate limiting, resource quotas."},
			{ItemText: "API5:2023 - Broken Function Level Authorization", DisplayOrder: 5, Notes: "Attempt to access admin functions as a regular user."},
			{ItemText: "API6:2023 - Unrestricted Access to Sensitive Business Flows", DisplayOrder: 6, Notes: "Identify and test critical business logic flows for authorization bypasses."},
			{ItemText: "API7:2023 - Server Side Request Forgery", DisplayOrder: 7, Notes: "Look for parameters that accept URLs or hostnames."},
			{ItemText: "API8:2023 - Security Misconfiguration", DisplayOrder: 8, Notes: "Check for default settings, verbose errors, unnecessary features enabled."},
			{ItemText: "API9:2023 - Improper Inventory Management", DisplayOrder: 9, Notes: "Identify all API versions and environments (dev, staging, prod)."},
			{ItemText: "API10:2023 - Unsafe Consumption of APIs", DisplayOrder: 10, Notes: "Analyze how the API consumes data from third-party services."},
		},
		"Tool Commands": {
			{ItemText: "Nmap - Basic Scan", ItemCommandText: "nmap -sV -sC <target_ip>", DisplayOrder: 1, Notes: "Service version detection and default scripts."},
			{ItemText: "Dirsearch - Common Directories", ItemCommandText: "dirsearch -u <target_url> -e php,html,js,txt -w /usr/share/wordlists/dirbuster/directory-list-2.3-medium.txt", DisplayOrder: 2, Notes: "Adjust wordlist path as needed."},
			{ItemText: "SQLMap - Test URL for SQLi", ItemCommandText: "sqlmap -u \"<target_url_with_params>\" --batch --dbs", DisplayOrder: 3, Notes: "Basic database enumeration."},
			{ItemText: "Subfinder - Enumerate Subdomains", ItemCommandText: "subfinder -d <target_domain>", DisplayOrder: 4, Notes: "Passive subdomain enumeration."},
			{ItemText: "Amass - Active Subdomain Enum", ItemCommandText: "amass enum -active -d <target_domain> -brute -w /path/to/subdomain_wordlist.txt", DisplayOrder: 5, Notes: "More comprehensive, active enumeration."},
		},
	}

	for _, t := range templates {
		res, err := DB.Exec("INSERT OR IGNORE INTO checklist_templates (name, description) VALUES (?, ?)", t.Name, t.Description)
		if err != nil {
			return fmt.Errorf("seeding template '%s': %w", t.Name, err)
		}
		templateID, _ := res.LastInsertId()

		if templateID == 0 {
			err := DB.QueryRow("SELECT id FROM checklist_templates WHERE name = ?", t.Name).Scan(&templateID)
			if err != nil {
				logger.Info("Could not retrieve existing template ID for '%s' during seeding: %v", t.Name, err)
				continue
			}
		}
		if templateID > 0 && itemsMap[t.Name] != nil {
			for _, item := range itemsMap[t.Name] {
				itemNotes := models.NullString(item.Notes)
				itemCommand := models.NullString(item.ItemCommandText)
				_, err := DB.Exec("INSERT OR IGNORE INTO checklist_template_items (template_id, item_text, item_command_text, notes, display_order) VALUES (?, ?, ?, ?, ?)", templateID, item.ItemText, itemCommand, itemNotes, item.DisplayOrder)
				if err != nil {
					logger.Info("Could not seed item '%s' for template '%s': %v", item.ItemText, t.Name, err)
				}
			}
		}
	}
	logger.Info("Initial checklist templates seeding attempted.")
	return nil
}

func GetChecklistItemsByTargetID(targetID int64) ([]models.TargetChecklistItem, error) {
	rows, err := DB.Query(`
		SELECT id, target_id, item_text, item_command_text, notes, is_completed, created_at, updated_at
		FROM target_checklist_items
		WHERE target_id = ?
		ORDER BY created_at ASC, id ASC
	`, targetID)
	if err != nil {
		return nil, fmt.Errorf("querying checklist items for target %d: %w", targetID, err)
	}
	defer rows.Close()

	var items []models.TargetChecklistItem
	for rows.Next() {
		var item models.TargetChecklistItem
		var notes sql.NullString
		var commandText sql.NullString
		if err := rows.Scan(&item.ID, &item.TargetID, &item.ItemText, &commandText, &notes, &item.IsCompleted, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning checklist item for target %d: %w", targetID, err)
		}
		item.Notes = notes
		item.ItemCommandText = commandText
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating checklist items for target %d: %w", targetID, err)
	}
	return items, nil
}

func GetChecklistItemByID(itemID int64) (models.TargetChecklistItem, error) {
	var item models.TargetChecklistItem
	var notes sql.NullString
	var commandText sql.NullString
	err := DB.QueryRow(`
		SELECT id, target_id, item_text, item_command_text, notes, is_completed, created_at, updated_at
		FROM target_checklist_items
		WHERE id = ?
	`, itemID).Scan(&item.ID, &item.TargetID, &item.ItemText, &commandText, &notes, &item.IsCompleted, &item.CreatedAt, &item.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return item, fmt.Errorf("checklist item with ID %d not found: %w", itemID, err)
		}
		return item, fmt.Errorf("querying checklist item %d: %w", itemID, err)
	}
	item.Notes = notes
	item.ItemCommandText = commandText
	return item, nil
}

func AddChecklistItem(item models.TargetChecklistItem) (int64, error) {
	stmt, err := DB.Prepare(`
		INSERT INTO target_checklist_items (target_id, item_text, item_command_text, notes, is_completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return 0, fmt.Errorf("preparing add checklist item statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(item.TargetID, item.ItemText, item.ItemCommandText, item.Notes, item.IsCompleted)
	if err != nil {
		return 0, fmt.Errorf("executing add checklist item statement for target %d: %w", item.TargetID, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert ID for checklist item: %w", err)
	}
	return id, nil
}

func UpdateChecklistItem(item models.TargetChecklistItem) error {
	stmt, err := DB.Prepare(`
		UPDATE target_checklist_items
		SET item_text = ?, item_command_text = ?, notes = ?, is_completed = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing update checklist item statement for item %d: %w", item.ID, err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(item.ItemText, item.ItemCommandText, item.Notes, item.IsCompleted, item.ID)
	if err != nil {
		return fmt.Errorf("executing update checklist item statement for item %d: %w", item.ID, err)
	}
	return nil
}

func DeleteChecklistItem(itemID int64) error {
	stmt, err := DB.Prepare("DELETE FROM target_checklist_items WHERE id = ?")
	if err != nil {
		return fmt.Errorf("preparing delete checklist item statement for item %d: %w", itemID, err)
	}
	defer stmt.Close()

	if _, err := stmt.Exec(itemID); err != nil {
		return fmt.Errorf("executing delete checklist item statement for item %d: %w", itemID, err)
	}
	return nil
}

func AddChecklistItemIfNotExists(targetID int64, itemText string, itemCommandText sql.NullString, notes sql.NullString) (int64, bool, error) {
	stmt, err := DB.Prepare(`
		INSERT OR IGNORE INTO target_checklist_items (target_id, item_text, item_command_text, notes, is_completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return 0, false, fmt.Errorf("preparing add checklist item if not exists statement: %w", err)
	}
	defer stmt.Close()
	result, err := stmt.Exec(targetID, itemText, itemCommandText, notes, false) // Always insert as not completed
	if err != nil {
		return 0, false, fmt.Errorf("executing add checklist item if not exists statement for target %d: %w", targetID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("getting rows affected for add checklist item if not exists: %w", err)
	}

	if rowsAffected > 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return 0, true, fmt.Errorf("getting last insert ID for new checklist item: %w", err)
		}
		return id, true, nil
	}

	return 0, false, nil
}

func CreateNote(note models.Note) (int64, error) {
	stmt, err := DB.Prepare(`
		INSERT INTO notes (title, content, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return 0, fmt.Errorf("preparing create note statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(note.Title, note.Content)
	if err != nil {
		return 0, fmt.Errorf("executing create note statement: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert ID for note: %w", err)
	}
	return id, nil
}

func GetNoteByID(noteID int64) (models.Note, error) {
	var note models.Note
	err := DB.QueryRow(`
		SELECT id, title, content, created_at, updated_at
		FROM notes
		WHERE id = ?
	`, noteID).Scan(&note.ID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return note, fmt.Errorf("note with ID %d not found: %w", noteID, err)
		}
		return note, fmt.Errorf("querying note %d: %w", noteID, err)
	}
	return note, nil
}

func GetAllNotesPaginated(limit int, offset int, sortByColumn string, sortOrder string) ([]models.Note, int64, error) {
	var notes []models.Note
	var totalRecords int64

	countQuery := "SELECT COUNT(*) FROM notes"
	err := DB.QueryRow(countQuery).Scan(&totalRecords)
	if err != nil {
		return nil, 0, fmt.Errorf("counting notes: %w", err)
	}

	if totalRecords == 0 {
		return notes, 0, nil
	}

	allowedSortColumns := map[string]bool{"title": true, "created_at": true, "updated_at": true, "id": true}
	if !allowedSortColumns[sortByColumn] {
		sortByColumn = "updated_at"
	}
	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		sortOrder = "DESC"
	}

	query := fmt.Sprintf(`SELECT id, title, content, created_at, updated_at
						   FROM notes
						   ORDER BY %s %s, id %s
						   LIMIT ? OFFSET ?`, sortByColumn, sortOrder, sortOrder)

	rows, err := DB.Query(query, limit, offset)
	if err != nil {
		return nil, totalRecords, fmt.Errorf("querying notes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var note models.Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, totalRecords, fmt.Errorf("scanning note row: %w", err)
		}
		notes = append(notes, note)
	}
	return notes, totalRecords, rows.Err()
}

func UpdateNote(note models.Note) error {
	stmt, err := DB.Prepare(`
		UPDATE notes
		SET title = ?, content = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("preparing update note statement for note %d: %w", note.ID, err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(note.Title, note.Content, note.ID)
	return err
}

func DeleteNote(noteID int64) error {
	_, err := DB.Exec("DELETE FROM notes WHERE id = ?", noteID)
	return err
}
