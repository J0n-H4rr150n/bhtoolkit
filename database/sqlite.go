package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func createTables() error {
	platformTable := `
    CREATE TABLE IF NOT EXISTS platforms (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE NOT NULL
    );`
	targetTable := `
    CREATE TABLE IF NOT EXISTS targets (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        platform_id INTEGER NOT NULL,
        slug TEXT UNIQUE,
        codename TEXT NOT NULL,
        link TEXT NOT NULL,
        notes TEXT,
        FOREIGN KEY(platform_id) REFERENCES platforms(id) ON DELETE CASCADE,
        UNIQUE (platform_id, codename) 
    );`
	scopeRulesTable := `
    CREATE TABLE IF NOT EXISTS scope_rules (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        target_id INTEGER NOT NULL,
        item_type TEXT NOT NULL CHECK(item_type IN ('domain', 'subdomain', 'ip_address', 'cidr', 'url_path')),
        pattern TEXT NOT NULL,
        is_in_scope BOOLEAN NOT NULL,
        is_wildcard BOOLEAN NOT NULL DEFAULT FALSE,
        description TEXT,
        FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE,
        UNIQUE (target_id, item_type, pattern, is_in_scope)
    );`
	httpTrafficLogTable := `
    CREATE TABLE IF NOT EXISTS http_traffic_log (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        target_id INTEGER, -- Mapped target ID
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
        request_method TEXT,
        request_url TEXT,
        request_http_version TEXT,
        request_headers TEXT,
        request_body BLOB,
        response_status_code INTEGER,
        response_reason_phrase TEXT,
        response_http_version TEXT,
        response_headers TEXT,
        response_body BLOB,
        response_content_type TEXT,
        response_body_size INTEGER,
        duration_ms INTEGER,
        client_ip TEXT,
        server_ip TEXT,
        is_https BOOLEAN,
        is_page_candidate BOOLEAN DEFAULT FALSE,
        notes TEXT,
        is_favorite BOOLEAN DEFAULT FALSE, -- Added for favorite logs
        FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE SET NULL
    );`
	webPagesTable := `
    CREATE TABLE IF NOT EXISTS web_pages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        target_id INTEGER NOT NULL,
        url TEXT NOT NULL,
        title TEXT,
        discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(target_id, url),
        FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
    );`
	apiEndpointsTable := `
    CREATE TABLE IF NOT EXISTS api_endpoints (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        target_id INTEGER NOT NULL,
        method TEXT NOT NULL,
        path_pattern TEXT NOT NULL,
        description TEXT,
        parameters_info TEXT,
        discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(target_id, method, path_pattern),
        FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
    );`
	pageApiRelationshipsTable := `
    CREATE TABLE IF NOT EXISTS page_api_relationships (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        web_page_id INTEGER NOT NULL,
        api_endpoint_id INTEGER NOT NULL,
        trigger_info TEXT,
        UNIQUE(web_page_id, api_endpoint_id),
        FOREIGN KEY(web_page_id) REFERENCES web_pages(id) ON DELETE CASCADE,
        FOREIGN KEY(api_endpoint_id) REFERENCES api_endpoints(id) ON DELETE CASCADE
    );`
	analysisResultsTable := `
    CREATE TABLE IF NOT EXISTS analysis_results (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        http_log_id INTEGER NOT NULL,
        analysis_type TEXT NOT NULL,
        result_data TEXT,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY(http_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE
    );`

	targetChecklistItemsTable := `
	CREATE TABLE IF NOT EXISTS target_checklist_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id INTEGER NOT NULL,
		item_text TEXT NOT NULL,
		notes TEXT,
		is_completed BOOLEAN NOT NULL DEFAULT FALSE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, -- Ensure item_text is unique per target
		UNIQUE (target_id, item_text),
		FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
	);`
	synackTargetsTable := `
	CREATE TABLE IF NOT EXISTS synack_targets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,          
		synack_target_id_str TEXT UNIQUE NOT NULL, 
		codename TEXT,                            
		organization_id TEXT,                     
		activated_at TEXT,                        
		name TEXT,                                
		category TEXT,                            
		outage_windows_raw TEXT,                  
		vulnerability_discovery_raw TEXT,         
		collaboration_criteria_raw TEXT,          
		raw_json_details TEXT,                    
		first_seen_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'new',       
		is_active BOOLEAN DEFAULT TRUE,           
		deactivated_at DATETIME,                  
		notes TEXT,                               
		analytics_last_fetched_at DATETIME        
	);`

	discoveredUrlsTable := `
	CREATE TABLE IF NOT EXISTS discovered_urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target_id INTEGER, -- Can be NULL if the source traffic log wasn't mapped yet
		http_traffic_log_id INTEGER NOT NULL,
		url TEXT NOT NULL,
		source_type TEXT NOT NULL, -- e.g., "jsluice", "html_href", "proxy_log"
		discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (http_traffic_log_id, url, source_type), -- Avoid duplicates from the same source log entry
		FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE SET NULL,
		FOREIGN KEY(http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE
	);`

	synackTargetAnalyticsCategoriesTable := `
	CREATE TABLE IF NOT EXISTS synack_target_analytics_categories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		synack_target_db_id INTEGER NOT NULL,
		category_name TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 0, -- Monetary fields removed
		FOREIGN KEY(synack_target_db_id) REFERENCES synack_targets(id) ON DELETE CASCADE,
		UNIQUE (synack_target_db_id, category_name)
	);`

	synackFindingsTable := `
	CREATE TABLE IF NOT EXISTS synack_findings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		synack_target_db_id INTEGER NOT NULL,
		synack_finding_id TEXT NOT NULL, -- The ID from Synack, e.g., "SF-XXXXX"
		title TEXT,
		category_name TEXT,
		severity TEXT,
		status TEXT,
		amount_paid REAL DEFAULT 0.0,
		vulnerability_url TEXT,
		reported_at DATETIME,
		closed_at DATETIME,
		raw_json_details TEXT,
		last_updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, -- Local DB update timestamp
		FOREIGN KEY(synack_target_db_id) REFERENCES synack_targets(id) ON DELETE CASCADE,
		UNIQUE (synack_target_db_id, synack_finding_id)
	);`

	appSettingsTable := `
	CREATE TABLE IF NOT EXISTS app_settings (
		key TEXT PRIMARY KEY NOT NULL,
		value TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	checklistTemplatesTable := `
	CREATE TABLE IF NOT EXISTS checklist_templates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		description TEXT
	);`

	checklistTemplateItemsTable := `
	CREATE TABLE IF NOT EXISTS checklist_template_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		template_id INTEGER NOT NULL,
		item_text TEXT NOT NULL,
		item_command_text TEXT, -- New column for the command
		notes TEXT,
		display_order INTEGER DEFAULT 0, -- Ensure this is unique per template
		UNIQUE (template_id, display_order), -- Consider if display_order alone is enough or if (template_id, item_text) should be unique
		FOREIGN KEY(template_id) REFERENCES checklist_templates(id) ON DELETE CASCADE
	);`

	tables := []string{
		platformTable, targetTable, scopeRulesTable, httpTrafficLogTable, targetChecklistItemsTable, // Added new table
		webPagesTable, apiEndpointsTable, pageApiRelationshipsTable, analysisResultsTable,
		synackTargetsTable, discoveredUrlsTable,
		appSettingsTable,
	}
	tables = append(tables, checklistTemplatesTable)
	tables = append(tables, checklistTemplateItemsTable)

	tables = append(tables, synackTargetAnalyticsCategoriesTable)
	tables = append(tables, synackFindingsTable)
	for _, tableSQL := range tables {
		if _, err := DB.Exec(tableSQL); err != nil {
			logger.Error("Failed to create table: %v\nSQL:\n%s", err, tableSQL)
			return fmt.Errorf("failed to create table: %w (SQL: %s)", err, tableSQL)
		}
	}

	logger.Info("Database tables created/verified successfully.")
	return nil
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

func GetInScopeRulesForTarget(targetID int64) ([]models.ScopeRule, error) {
	rows, err := DB.Query(`SELECT id, item_type, pattern, is_in_scope, is_wildcard, description 
                           FROM scope_rules 
                           WHERE target_id = ? AND is_in_scope = TRUE 
                           ORDER BY id ASC`, targetID)
	if err != nil {
		return nil, fmt.Errorf("querying in-scope rules for target %d: %w", targetID, err)
	}
	defer rows.Close()

	var rules []models.ScopeRule
	for rows.Next() {
		var sr models.ScopeRule
		sr.TargetID = targetID
		if err := rows.Scan(&sr.ID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &sr.Description); err != nil {
			return nil, fmt.Errorf("scanning in-scope rule for target %d: %w", targetID, err)
		}
		rules = append(rules, sr)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating in-scope rules for target %d: %w", targetID, err)
	}
	return rules, nil
}

func GetSetting(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM app_settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("querying setting %s: %w", key, err)
	}
	return value, nil
}

func SetSetting(key string, value string) error {
	stmt, err := DB.Prepare(`
		INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP;
	`)
	if err != nil {
		return fmt.Errorf("preparing set setting statement for key %s: %w", key, err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(key, value)
	if err != nil {
		return fmt.Errorf("executing set setting statement for key %s: %w", key, err)
	}
	logger.Info("Setting '%s' updated in database.", key)
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
		parsedTime, parseErr = time.Parse("2006-01-02 15:04:05", fetchedAt.String)
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
		valueToStore = nil // Pass nil to store SQL NULL
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

	allowedSortColumns := map[string]bool{
		"category_name": true,
		"count":         true,
	}
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

	if err = rows.Err(); err != nil {
		logger.Error("GetSynackTargetAnalyticsPaginated: Error iterating analytics rows for target_db_id %d: %v", targetDbID, err)
		return nil, totalRecords, fmt.Errorf("iterating analytics rows for target %d: %w", targetDbID, err)
	}

	return analytics, totalRecords, nil
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

	logger.Debug("ListAllSynackAnalyticsPaginated: Received sortByColumn='%s', sortOrder='%s'", sortByColumn, sortOrder)

	allowedSortColumns := map[string]string{
		"target_codename": "st.codename",
		"category_name":   "stac.category_name",
		"count":           "stac.count",
	}
	dbSortColumnActual, ok := allowedSortColumns[sortByColumn]
	logger.Debug("ListAllSynackAnalyticsPaginated: Map lookup for sortByColumn='%s': found=%t, dbColumnResult='%s'", sortByColumn, ok, dbSortColumnActual)

	if !ok {
		logger.Info("ListAllSynackAnalyticsPaginated: sortByColumn='%s' not found in allowedSortColumns. Defaulting to 'st.codename'. Input sortByColumn will be treated as 'target_codename' for COLLATE logic.", sortByColumn)
		dbSortColumnActual = "st.codename"
		sortByColumn = "target_codename"
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

	logger.Debug("ListAllSynackAnalyticsPaginated: Executing query: %s", query)
	logger.Debug("ListAllSynackAnalyticsPaginated: Query args: limit=%d, offset=%d", limit, offset)

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

	logger.Debug("GetSynackTargetFindingsPaginated: TargetDBID: %d, Limit: %d, Offset: %d, SortBy: '%s', SortOrder: '%s', TotalRecordsFromCount: %d",
		targetDbID, limit, offset, sortByColumn, sortOrder, totalRecords)

	allowedSortColumns := map[string]string{
		"synack_finding_id": "synack_finding_id",
		"title":             "title",
		"category_name":     "category_name",
		"severity":          "severity",
		"status":            "status",
		"amount_paid":       "amount_paid",
		"vulnerability_url": "vulnerability_url",
		"reported_at":       "reported_at",
		"closed_at":         "closed_at",
		"id":                "id",
	}
	dbSortColumn, ok := allowedSortColumns[sortByColumn]
	actualSortByColumnForCollate := sortByColumn
	if !ok {
		dbSortColumn = "reported_at"
		actualSortByColumnForCollate = "reported_at"
	}

	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		if actualSortByColumnForCollate == "title" || actualSortByColumnForCollate == "category_name" || actualSortByColumnForCollate == "synack_finding_id" || actualSortByColumnForCollate == "severity" || actualSortByColumnForCollate == "status" {
			sortOrder = "ASC"
		} else {
			sortOrder = "DESC"
		}
	}

	dbSortColumnSQL := dbSortColumn
	textSortColumns := map[string]bool{
		"synack_finding_id": true,
		"title":             true,
		"category_name":     true,
		"severity":          true,
		"status":            true,
		"vulnerability_url": true,
	}
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

	logger.Debug("GetSynackTargetFindingsPaginated: Executing query: %s", query)
	logger.Debug("GetSynackTargetFindingsPaginated: Query args: targetDbID=%d, limit=%d, offset=%d", targetDbID, limit, offset)

	rows, err := DB.Query(query, targetDbID, limit, offset)
	if err != nil {
		logger.Error("GetSynackTargetFindingsPaginated: Error querying findings for target_db_id %d: %v", targetDbID, err)
		return nil, totalRecords, fmt.Errorf("querying findings for target %d: %w", targetDbID, err)
	}
	defer rows.Close()

	var rowCount int = 0
	for rows.Next() {
		rowCount++
		logger.Debug("GetSynackTargetFindingsPaginated: Processing row %d", rowCount)
		var f models.SynackFinding
		var reportedAtScannable sql.NullTime
		var closedAtScannable sql.NullTime
		var amountPaidScannable sql.NullFloat64

		err := rows.Scan(
			&f.DBID, &f.SynackTargetDBID, &f.SynackFindingID, &f.Title, &f.CategoryName,
			&f.Severity, &f.Status, &amountPaidScannable, &f.VulnerabilityURL,
			&reportedAtScannable, &closedAtScannable, &f.RawJSONDetails,
		)
		if err != nil {
			logger.Error("GetSynackTargetFindingsPaginated: Error scanning finding row %d for target_db_id %d: %v. SQL Query was: %s", rowCount, targetDbID, err, query)
			return nil, totalRecords, fmt.Errorf("scanning finding row for target %d: %w", targetDbID, err)
		}

		if reportedAtScannable.Valid {
			t := reportedAtScannable.Time
			f.ReportedAt = &t
		} else {
			f.ReportedAt = nil
		}
		if closedAtScannable.Valid {
			t := closedAtScannable.Time
			f.ClosedAt = &t
		} else {
			f.ClosedAt = nil
		}
		if amountPaidScannable.Valid {
			f.AmountPaid = amountPaidScannable.Float64
		} else {
			f.AmountPaid = 0.0
		}

		logger.Debug("GetSynackTargetFindingsPaginated: Scanned row %d: FindingID='%s', Title_len=%d, Category='%s', Status='%s', ReportedAtValid=%t, ClosedAtValid=%t, AmountPaidValid=%t, AmountPaidValue=%.2f", rowCount, f.SynackFindingID, len(f.Title), f.CategoryName, f.Status, reportedAtScannable.Valid, closedAtScannable.Valid, amountPaidScannable.Valid, f.AmountPaid)
		findings = append(findings, f)
	}
	if err := rows.Err(); err != nil {
		logger.Error("GetSynackTargetFindingsPaginated: Error after iterating rows for target_db_id %d: %v", targetDbID, err)
		return findings, totalRecords, fmt.Errorf("iterating rows for target %d: %w", targetDbID, err)
	}
	logger.Debug("GetSynackTargetFindingsPaginated: Finished iterating rows. Total rows processed: %d. Total findings collected: %d.", rowCount, len(findings))
	return findings, totalRecords, nil
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

	logger.Debug("ListSynackTargetsPaginated: Limit: %d, Offset: %d, SortBy: '%s', SortOrder: '%s', IsActiveFilter: %v, TotalRecordsFromCount: %d",
		limit, offset, sortByColumn, sortOrder, filterIsActive, totalRecords)

	allowedSortColumns := map[string]string{
		"synack_target_id_str": "st.synack_target_id_str",
		"codename":             "st.codename",
		"name":                 "st.name",
		"status":               "st.status",
		"last_seen_timestamp":  "st.last_seen_timestamp",
		"findings_count":       "(SELECT COUNT(sf.id) FROM synack_findings sf WHERE sf.synack_target_db_id = st.id)",
		"id":                   "st.id",
	}
	dbSortColumn, ok := allowedSortColumns[sortByColumn]
	actualSortByColumnForCollate := sortByColumn
	if !ok {
		dbSortColumn = "st.last_seen_timestamp"
		actualSortByColumnForCollate = "last_seen_timestamp"
	}

	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		if actualSortByColumnForCollate == "synack_target_id_str" || actualSortByColumnForCollate == "codename" || actualSortByColumnForCollate == "name" || actualSortByColumnForCollate == "status" {
			sortOrder = "ASC"
		} else {
			sortOrder = "DESC"
		}
	}

	dbSortColumnSQL := dbSortColumn
	textSortColumns := map[string]bool{
		"synack_target_id_str": true,
		"codename":             true,
		"name":                 true,
		"status":               true,
	}
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

	logger.Debug("ListSynackTargetsPaginated: Executing query: %s", query)
	logger.Debug("ListSynackTargetsPaginated: Query args: %v", queryArgs)

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
