package database

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	// "regexp" // No longer needed for simple difficulty extraction from old notes for Juice Shop
	"sort"
	"strings" // Added for time.Now() in seedInitialChecklistTemplates
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
		// If a migration fails, the database is considered "dirty".
		// We can try to get the version to help the user identify the problematic file.
		version, dirty, vErr := m.Version()
		if vErr != nil {
			logger.Error("Failed to apply migrations and could not determine migration version: %v. Original migration error: %v", vErr, err)
			return fmt.Errorf("failed to apply migrations: %w (version check failed: %v)", vErr, vErr)
		}
		if dirty {
			logger.Error("Database is in a dirty state. The last attempted migration was version %d. Please check the migration file for this version for errors.", version)
		}
		logger.Error("Failed to apply migrations (raw error): %v", err)
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	logger.Info("Database migrations applied successfully (or no changes).")
	if err := seedInitialChecklistTemplates(); err != nil {
		return fmt.Errorf("failed to seed checklist templates: %w", err)
	}
	if err := seedInitialVulnerabilityTypes(); err != nil {
		return fmt.Errorf("failed to seed vulnerability types: %w", err)
	}
	// Seed tags after checklist templates
	return seedTagsFromJSON()
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
		// OWASP Juice Shop Challenges template will be added dynamically if data is loaded
	}

	type challengeItem struct {
		ItemText        string
		ItemCommandText string
		Notes           string
		DisplayOrder    int
		Difficulty      int // For sorting
	}

	// Struct to unmarshal the provided JSON data for Juice Shop challenges
	type JuiceShopChallengeData struct {
		Name          string   `json:"name"`
		Category      string   `json:"category"`
		Description   string   `json:"description"`
		Difficulty    int      `json:"difficulty"`
		Hint          string   `json:"hint"`
		HintURL       string   `json:"hintUrl"`
		MitigationURL *string  `json:"mitigationUrl"` // Use pointer for nullable string
		Key           string   `json:"key"`
		Tags          []string `json:"tags,omitempty"`
		DisabledEnv   []string `json:"disabledEnv,omitempty"`
		Tutorial      *struct {
			Order int `json:"order"`
		} `json:"tutorial,omitempty"` // Use pointer for nullable object
	}

	itemsMap := map[string][]challengeItem{
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
		// "OWASP Juice Shop Challenges" will be populated from JSON
	}

	// --- Enrich OWASP Juice Shop Challenges Notes ---
	jsonFilePath := filepath.Join("database", "seed", "owasp_juice_shop_challenges.json")
	jsonDataBytes, errFile := ioutil.ReadFile(jsonFilePath)
	if errFile != nil {
		logger.Error("seedInitialChecklistTemplates: Failed to read owasp_juice_shop_challenges.json: %v. Skipping Juice Shop template.", errFile)
	} else {
		var allChallengeData []JuiceShopChallengeData
		err := json.Unmarshal(jsonDataBytes, &allChallengeData)
		if err != nil {
			logger.Error("seedInitialChecklistTemplates: Failed to unmarshal Juice Shop challenges JSON: %v. Skipping Juice Shop template.", err)
		} else {
			var enrichedJuiceShopItems []challengeItem
			for _, detailedData := range allChallengeData {
				var newNotesBuilder bytes.Buffer
				newNotesBuilder.WriteString(fmt.Sprintf("Category: %s\n", detailedData.Category))
				newNotesBuilder.WriteString(fmt.Sprintf("Difficulty: %d\n", detailedData.Difficulty))
				if detailedData.Description != "" {
					newNotesBuilder.WriteString(fmt.Sprintf("Description: %s\n", detailedData.Description))
				}
				if detailedData.Hint != "" {
					newNotesBuilder.WriteString(fmt.Sprintf("Hint: %s\n", detailedData.Hint))
				}
				if detailedData.HintURL != "" {
					newNotesBuilder.WriteString(fmt.Sprintf("Hint URL: %s\n", detailedData.HintURL))
				}
				if detailedData.MitigationURL != nil && *detailedData.MitigationURL != "" {
					newNotesBuilder.WriteString(fmt.Sprintf("Mitigation URL: %s\n", *detailedData.MitigationURL))
				}
				if len(detailedData.Tags) > 0 {
					newNotesBuilder.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(detailedData.Tags, ", ")))
				}
				if detailedData.Key != "" {
					newNotesBuilder.WriteString(fmt.Sprintf("Key: %s\n", detailedData.Key))
				}
				if len(detailedData.DisabledEnv) > 0 {
					newNotesBuilder.WriteString(fmt.Sprintf("Disabled Env: %s\n", strings.Join(detailedData.DisabledEnv, ", ")))
				}
				if detailedData.Tutorial != nil {
					newNotesBuilder.WriteString(fmt.Sprintf("Tutorial Order: %d\n", detailedData.Tutorial.Order))
				}

				enrichedJuiceShopItems = append(enrichedJuiceShopItems, challengeItem{
					ItemText:   detailedData.Name,
					Notes:      strings.TrimSpace(newNotesBuilder.String()),
					Difficulty: detailedData.Difficulty,
				})
			}

			// Add the "OWASP Juice Shop Challenges" template definition if not already present
			juiceShopTemplateName := "OWASP Juice Shop Challenges"
			foundJuiceShopTemplate := false
			for _, t := range templates {
				if t.Name == juiceShopTemplateName {
					foundJuiceShopTemplate = true
					break
				}
			}
			if !foundJuiceShopTemplate {
				templates = append(templates, models.ChecklistTemplate{
					Name:        juiceShopTemplateName,
					Description: sql.NullString{String: "A comprehensive list of OWASP Juice Shop challenges with detailed information, sourced from official data.", Valid: true},
				})
			}

			itemsMap[juiceShopTemplateName] = enrichedJuiceShopItems

			// Sort the enriched challenges
			sort.SliceStable(itemsMap[juiceShopTemplateName], func(i, j int) bool {
				itemI := itemsMap[juiceShopTemplateName][i]
				itemJ := itemsMap[juiceShopTemplateName][j]
				if itemI.Difficulty != itemJ.Difficulty {
					return itemI.Difficulty < itemJ.Difficulty
				}
				return strings.ToLower(itemI.ItemText) < strings.ToLower(itemJ.ItemText)
			})
			logger.Info("Successfully loaded and processed %d OWASP Juice Shop challenges from JSON.", len(enrichedJuiceShopItems))
		}
	}
	// --- End of Enrichment ---

	for _, t := range templates {
		res, err := DB.Exec("INSERT OR IGNORE INTO checklist_templates (name, description) VALUES (?, ?)", t.Name, t.Description)
		if err != nil {
			return fmt.Errorf("seeding template '%s': %w", t.Name, err)
		}
		templateID, _ := res.LastInsertId()

		if templateID == 0 { // If IGNORE happened, get existing ID
			logger.Info("Template '%s' likely already exists, attempting to fetch its ID.", t.Name)
			err := DB.QueryRow("SELECT id FROM checklist_templates WHERE name = ?", t.Name).Scan(&templateID)
			if err != nil {
				logger.Error("Could not retrieve existing template ID for '%s' during seeding: %v. Skipping items for this template.", t.Name, err)
				continue
			}
		}

		if templateID > 0 {
			itemsForTemplate, ok := itemsMap[t.Name]
			if !ok || itemsForTemplate == nil { // Check if key exists and if slice is not nil
				continue
			}
			for i, item := range itemsForTemplate {
				itemNotes := models.NullString(item.Notes) // Use the enriched notes
				itemCommand := models.NullString(item.ItemCommandText)
				currentDisplayOrder := i + 1 // Re-assign display order after sorting
				_, err := DB.Exec("INSERT OR IGNORE INTO checklist_template_items (template_id, item_text, item_command_text, notes, display_order) VALUES (?, ?, ?, ?, ?)", templateID, item.ItemText, itemCommand, itemNotes, currentDisplayOrder)
				if err != nil {
					// Log error but continue seeding other items/templates
					logger.Info("Could not seed item '%s' for template '%s': %v", item.ItemText, t.Name, err)
				}
			}
		}
	}
	logger.Info("Initial checklist templates seeding attempted.")
	return nil
}

// seedTagsFromJSON reads tags from the seed/tags.json file and adds them to the database.
func seedTagsFromJSON() error {
	jsonFilePath := filepath.Join("database", "seed", "tags.json")
	jsonDataBytes, errFile := ioutil.ReadFile(jsonFilePath)
	if errFile != nil {
		logger.Error("seedTagsFromJSON: Failed to read tags.json: %v. Skipping tag seeding.", errFile)
		return nil // Non-fatal, allow app to start without seeded tags
	}

	var seedTags []struct {
		Name  string `json:"name"` // This is the user-facing name of the tag
		Tag   string `json:"tag"`  // This is the short, unique identifier (actual tag value)
		Color string `json:"color"`
		// Other fields like type, category, value are for the auto-tagging logic, not directly stored in 'tags' table
	}

	err := json.Unmarshal(jsonDataBytes, &seedTags)
	if err != nil {
		logger.Error("seedTagsFromJSON: Failed to unmarshal tags.json: %v. Skipping tag seeding.", err)
		return nil // Non-fatal
	}

	var createdCount, skippedCount int
	for _, seedTag := range seedTags {
		tagToCreate := models.Tag{
			Name:  seedTag.Tag, // Use the "tag" field from JSON as the unique tag name
			Color: sql.NullString{String: seedTag.Color, Valid: seedTag.Color != ""},
		}
		_, createErr := CreateTag(tagToCreate) // CreateTag handles if it already exists
		if createErr != nil {
			logger.Warn("seedTagsFromJSON: Could not create/seed tag '%s': %v", tagToCreate.Name, createErr)
			skippedCount++
		} else {
			createdCount++
		}
	}

	logger.Info("Tag seeding from JSON complete. Created: %d, Skipped/Existing: %d", createdCount, skippedCount)
	return nil
}

func seedInitialVulnerabilityTypes() error {
	types := []models.VulnerabilityType{
		{Name: "SQL Injection (SQLi)", Description: models.NullString("Injecting SQL queries via input data to manipulate database commands.")},
		{Name: "Cross-Site Scripting (XSS) - Reflected", Description: models.NullString("Injecting malicious scripts into web pages viewed by other users, where the script is reflected off a web server.")},
		{Name: "Cross-Site Scripting (XSS) - Stored", Description: models.NullString("Injecting malicious scripts into a web application''s storage, which are then served to other users.")},
		{Name: "Cross-Site Scripting (XSS) - DOM-based", Description: models.NullString("Manipulating the Document Object Model (DOM) in the victim''s browser to execute malicious scripts.")},
		{Name: "XML External Entity (XXE) Injection", Description: models.NullString("Exploiting XML parsers that process external entities, potentially leading to information disclosure or SSRF.")},
		{Name: "Server-Side Request Forgery (SSRF)", Description: models.NullString("Forcing an application to make requests to an unintended location or an internal resource.")},
		{Name: "Insecure Direct Object References (IDOR)", Description: models.NullString("Accessing unauthorized data by manipulating object references (e.g., user IDs, file names).")},
		{Name: "Broken Access Control", Description: models.NullString("Failures in enforcing restrictions on what authenticated users are allowed to do.")},
		{Name: "Security Misconfiguration", Description: models.NullString("Incorrectly configured security settings, default credentials, or overly verbose error messages.")},
		{Name: "Insecure Deserialization", Description: models.NullString("Exploiting flaws in the deserialization of untrusted data, leading to remote code execution or other impacts.")},
		{Name: "Sensitive Data Exposure", Description: models.NullString("Exposing sensitive information such as PII, credentials, or financial data due to lack of encryption or other protections.")},
		{Name: "Authentication Bypass", Description: models.NullString("Gaining unauthorized access by circumventing authentication mechanisms.")},
		{Name: "Authorization Bypass", Description: models.NullString("Gaining access to functionalities or resources that should not be permitted for the current user role.")},
		{Name: "Remote Code Execution (RCE)", Description: models.NullString("Ability to execute arbitrary code on the target server.")},
		{Name: "Local File Inclusion (LFI)", Description: models.NullString("Including local files on the server, potentially exposing sensitive information or leading to RCE.")},
		{Name: "Remote File Inclusion (RFI)", Description: models.NullString("Including remote files on the server, often leading to RCE.")},
		{Name: "Directory Traversal / Path Traversal", Description: models.NullString("Accessing files and directories stored outside the web root folder.")},
		{Name: "Command Injection", Description: models.NullString("Executing arbitrary commands on the host operating system via a vulnerable application.")},
		{Name: "Cross-Site Request Forgery (CSRF)", Description: models.NullString("Forcing an end user to execute unwanted actions on a web application in which they are currently authenticated.")},
		{Name: "Open Redirect", Description: models.NullString("Redirecting users to an arbitrary external URL, often used in phishing attacks.")},
		{Name: "Subdomain Takeover", Description: models.NullString("Gaining control over a subdomain that points to a de-provisioned third-party service.")},
		{Name: "Information Disclosure", Description: models.NullString("Revealing sensitive information that could aid an attacker (e.g., stack traces, internal paths, user data).")},
		{Name: "Weak Cryptography", Description: models.NullString("Use of outdated or weak cryptographic algorithms or improper implementation of cryptography.")},
		{Name: "Denial of Service (DoS)", Description: models.NullString("Overwhelming a system with traffic or requests to make it unavailable to legitimate users.")},
		{Name: "Business Logic Flaws", Description: models.NullString("Exploiting flaws in the design and implementation of the application''s business logic.")},
		{Name: "Race Conditions", Description: models.NullString("Exploiting timing discrepancies in the execution of concurrent operations.")},
		{Name: "HTTP Request Smuggling", Description: models.NullString("Interfering with the way a sequence of HTTP requests are processed by one or more HTTP devices.")},
		{Name: "Web Cache Deception", Description: models.NullString("Tricking a web cache to store and serve sensitive user-specific content to other users.")},
		{Name: "Prototype Pollution (Client-Side)", Description: models.NullString("Injecting properties into `Object.prototype` in JavaScript, leading to XSS or other vulnerabilities.")},
		{Name: "Prototype Pollution (Server-Side - Node.js)", Description: models.NullString("Injecting properties into `Object.prototype` in Node.js, potentially leading to RCE or other vulnerabilities.")},
		{Name: "NoSQL Injection", Description: models.NullString("Injecting NoSQL database queries via input data.")},
		{Name: "LDAP Injection", Description: models.NullString("Injecting LDAP queries via input data.")},
		{Name: "Template Injection (SSTI)", Description: models.NullString("Injecting template language syntax into a template, leading to RCE or information disclosure.")},
		{Name: "Insecure File Upload", Description: models.NullString("Allowing upload of malicious files that can be executed on the server or client-side.")},
		{Name: "Missing HTTP Security Headers", Description: models.NullString("Absence of security headers like CSP, HSTS, X-Frame-Options, X-Content-Type-Options.")},
		{Name: "CORS Misconfiguration", Description: models.NullString("Improperly configured Cross-Origin Resource Sharing policies, allowing unauthorized cross-origin access.")},
		{Name: "GraphQL Injection / Batching Attacks", Description: models.NullString("Exploiting GraphQL implementations through malicious queries or batching.")},
		{Name: "OAuth/OIDC Misconfigurations", Description: models.NullString("Flaws in OAuth 2.0 or OpenID Connect implementations leading to account takeover or information disclosure.")},
		{Name: "Mass Assignment", Description: models.NullString("Binding client-controlled data to model properties or internal objects without proper validation or filtering.")},
	}

	for _, vt := range types {
		var existingID int64
		err := DB.QueryRow("SELECT id FROM vulnerability_types WHERE name = ?", vt.Name).Scan(&existingID)
		if err == nil {
			logger.Info("Vulnerability type '%s' already exists (ID: %d), skipping.", vt.Name, existingID)
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("checking for existing vulnerability type '%s': %w", vt.Name, err)
		}

		stmt, err := DB.Prepare(`
			INSERT INTO vulnerability_types (name, description, created_at, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`)
		if err != nil {
			return fmt.Errorf("preparing insert vulnerability type statement for '%s': %w", vt.Name, err)
		}
		defer stmt.Close()

		res, err := stmt.Exec(vt.Name, vt.Description)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
				// Known issue: SQLite unique constraint errors don't automatically skip.
				// Log the skip and continue.
				logger.Warn("Skipping existing vulnerability type (unique constraint): %s", vt.Name)
			} else {
				return fmt.Errorf("executing insert vulnerability type statement for '%s': %w", vt.Name, err)
			}
		} else {
			id, err := res.LastInsertId()
			if err != nil {
				logger.Error("seedInitialVulnerabilityTypes: Could not get last insert id: %v", err)
			} else {
				logger.Info("Inserted initial vulnerability type: ID %d, Name '%s'", id, vt.Name)
			}
		}
	}
	// Ensure this function is called by InitDB
	// It's currently missing from InitDB in the provided context.
	// You will need to add `if err := seedInitialVulnerabilityTypes(); err != nil { return fmt.Errorf("failed to seed vulnerability types: %w", err) }` to InitDB.
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

// GetChecklistItemsByTargetIDPaginated retrieves a paginated, sorted, and filtered list of checklist items for a target.
// It returns the items, total records matching filters, total completed records matching filters, and an error.
func GetChecklistItemsByTargetIDPaginated(targetID int64, limit int, offset int, sortBy string, sortOrder string, filter string, showIncompleteOnly bool) ([]models.TargetChecklistItem, int64, int64, error) {
	var items []models.TargetChecklistItem
	var totalCompletedRecordsForFilter int64
	var totalRecords int64

	// Validate sortBy column to prevent SQL injection
	allowedSortColumns := map[string]string{
		"id":                "id",
		"item_text":         "LOWER(item_text)", // Sort case-insensitively
		"item_command_text": "LOWER(item_command_text)",
		"notes":             "LOWER(notes)",
		"is_completed":      "is_completed",
		"created_at":        "created_at",
		"updated_at":        "updated_at",
	}
	dbSortBy, ok := allowedSortColumns[sortBy]
	if !ok {
		dbSortBy = "created_at" // Default sort column
	}

	if strings.ToUpper(sortOrder) != "ASC" && strings.ToUpper(sortOrder) != "DESC" {
		sortOrder = "ASC"
	}

	var whereClauses []string
	var args []interface{}

	// Assuming the table `target_checklist_items` is the primary table here and not aliased.
	// If it were aliased (e.g., "tci"), you'd use "tci.target_id = ?"
	// For now, assuming no alias is needed for target_id in this specific query context.
	// Explicitly state the table name for clarity, even if not strictly ambiguous in this specific query.
	// This helps if the query is ever expanded or combined.
	whereClauses = append(whereClauses, "target_checklist_items.target_id = ?")
	args = append(args, targetID)

	if filter != "" {
		filterPattern := "%" + strings.ToLower(filter) + "%" // Filter case-insensitively
		whereClauses = append(whereClauses, "(LOWER(item_text) LIKE ? OR LOWER(item_command_text) LIKE ? OR LOWER(notes) LIKE ?)")
		args = append(args, filterPattern, filterPattern, filterPattern)
	}

	if showIncompleteOnly {
		whereClauses = append(whereClauses, "is_completed = 0") // Or is_completed = FALSE depending on SQLite version/bool handling
	}

	whereCondition := ""
	if len(whereClauses) > 0 {
		whereCondition = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Use the table name in COUNT for clarity, though COUNT(*) would also work.
	countQuery := fmt.Sprintf("SELECT COUNT(target_checklist_items.id) FROM target_checklist_items %s", whereCondition)
	err := DB.QueryRow(countQuery, args...).Scan(&totalRecords)
	if err != nil {
		logger.Error("GetChecklistItemsByTargetIDPaginated: Error counting total records for target %d: %v", targetID, err)
		return nil, 0, 0, fmt.Errorf("counting total checklist items for target %d: %w", targetID, err)
	}

	if totalRecords == 0 {
		return items, 0, 0, nil
	}

	// Count completed items matching the filter (excluding showIncompleteOnly for this specific count)
	completedWhereClauses := []string{"target_checklist_items.target_id = ?", "is_completed = 1"} // Start with target_id and is_completed
	completedArgs := []interface{}{targetID}
	if filter != "" { // Apply text filter if present
		filterPattern := "%" + strings.ToLower(filter) + "%"
		completedWhereClauses = append(completedWhereClauses, "(LOWER(item_text) LIKE ? OR LOWER(item_command_text) LIKE ? OR LOWER(notes) LIKE ?)")
		completedArgs = append(completedArgs, filterPattern, filterPattern, filterPattern)
	}
	completedCountQuery := fmt.Sprintf("SELECT COUNT(target_checklist_items.id) FROM target_checklist_items WHERE %s", strings.Join(completedWhereClauses, " AND "))
	DB.QueryRow(completedCountQuery, completedArgs...).Scan(&totalCompletedRecordsForFilter) // Error handling for this can be added if critical

	queryArgs := make([]interface{}, len(args)) // Args for fetching the actual items
	copy(queryArgs, args)
	queryArgs = append(queryArgs, limit, offset)

	query := fmt.Sprintf(`
		SELECT id, target_id, item_text, item_command_text, notes, is_completed, created_at, updated_at
		FROM target_checklist_items  -- No alias needed here as it's the only table
		%s
		ORDER BY %s %s, id %s
		LIMIT ? OFFSET ?
	`, whereCondition, dbSortBy, sortOrder, sortOrder)

	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		logger.Error("GetChecklistItemsByTargetIDPaginated: Error querying for target %d: %v. Query: %s, Args: %v", targetID, err, query, queryArgs)
		return nil, totalRecords, totalCompletedRecordsForFilter, fmt.Errorf("querying checklist items for target %d: %w", targetID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var item models.TargetChecklistItem
		var notes sql.NullString
		var commandText sql.NullString
		if err := rows.Scan(&item.ID, &item.TargetID, &item.ItemText, &commandText, &notes, &item.IsCompleted, &item.CreatedAt, &item.UpdatedAt); err != nil {
			logger.Error("GetChecklistItemsByTargetIDPaginated: Error scanning row for target %d: %v", targetID, err)
			return nil, totalRecords, totalCompletedRecordsForFilter, fmt.Errorf("scanning checklist item for target %d: %w", targetID, err)
		}
		item.Notes = notes
		item.ItemCommandText = commandText
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		logger.Error("GetChecklistItemsByTargetIDPaginated: Error iterating rows for target %d: %v", targetID, err)
		return nil, totalRecords, totalCompletedRecordsForFilter, fmt.Errorf("iterating checklist items for target %d: %w", targetID, err)
	}
	return items, totalRecords, totalCompletedRecordsForFilter, nil
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

	var orderByClause string
	switch sortByColumn {
	case "title":
		orderByClause = "ORDER BY title " + sortOrder + ", id " + sortOrder
	case "created_at":
		orderByClause = "ORDER BY created_at " + sortOrder + ", id " + sortOrder
	case "updated_at":
		orderByClause = "ORDER BY updated_at " + sortOrder + ", id " + sortOrder
	case "id":
		orderByClause = "ORDER BY id " + sortOrder
	default:
		orderByClause = "ORDER BY updated_at DESC, id DESC"
	}

	query := fmt.Sprintf("SELECT id, title, content, created_at, updated_at FROM notes %s LIMIT ? OFFSET ?", orderByClause)
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

func CopyAllTemplateItemsToTarget(templateID int64, targetID int64) (int64, error) {
	tx, err := DB.Begin()
	if err != nil {
		logger.Error("CopyAllTemplateItemsToTarget: Failed to begin transaction for template %d to target %d: %v", templateID, targetID, err)
		return 0, fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT item_text, item_command_text, notes
		FROM checklist_template_items
		WHERE template_id = ?
		ORDER BY display_order ASC
	`, templateID)
	if err != nil {
		logger.Error("CopyAllTemplateItemsToTarget: Error querying items for template %d: %v", templateID, err)
		return 0, fmt.Errorf("querying items for template %d: %w", templateID, err)
	}
	defer rows.Close()

	var itemsCopiedCount int64 = 0
	stmt, errPrep := tx.Prepare(`
		INSERT OR IGNORE INTO target_checklist_items (target_id, item_text, item_command_text, notes, is_completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if errPrep != nil {
		return itemsCopiedCount, fmt.Errorf("preparing add checklist item statement (tx): %w", errPrep)
	}
	defer stmt.Close()

	for rows.Next() {
		var itemText string
		var itemCommandText sql.NullString
		var notes sql.NullString
		if errScan := rows.Scan(&itemText, &itemCommandText, &notes); errScan != nil {
			logger.Error("CopyAllTemplateItemsToTarget: Error scanning template item for template %d: %v", templateID, errScan)
			return itemsCopiedCount, fmt.Errorf("scanning template item: %w", errScan)
		}

		result, errExec := stmt.Exec(targetID, itemText, itemCommandText, notes, false) // Always insert as not completed
		if errExec != nil {
			// Don't close stmt here, it's deferred for the whole function
			return itemsCopiedCount, fmt.Errorf("executing add checklist item statement for target %d (tx): %w", targetID, errExec)
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			itemsCopiedCount++
		}
	}

	if err = rows.Err(); err != nil {
		logger.Error("CopyAllTemplateItemsToTarget: Error iterating template items for template %d: %v", templateID, err)
		return itemsCopiedCount, fmt.Errorf("iterating template items: %w", err)
	}

	if err = tx.Commit(); err != nil {
		logger.Error("CopyAllTemplateItemsToTarget: Failed to commit transaction for template %d to target %d: %v", templateID, targetID, err)
		return itemsCopiedCount, fmt.Errorf("committing transaction: %w", err)
	}

	logger.Info("Successfully copied %d items from template %d to target %d.", itemsCopiedCount, templateID, targetID)
	return itemsCopiedCount, nil
}

// DeleteAllChecklistItemsForTarget removes all checklist items associated with a given target_id.
func DeleteAllChecklistItemsForTarget(targetID int64) error {
	stmt, err := DB.Prepare("DELETE FROM target_checklist_items WHERE target_id = ?")
	if err != nil {
		return fmt.Errorf("preparing delete all checklist items statement for target %d: %w", targetID, err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(targetID)
	if err != nil {
		return fmt.Errorf("executing delete all checklist items for target %d: %w", targetID, err)
	}
	rowsAffected, _ := result.RowsAffected()
	logger.Info("Deleted %d checklist items for target ID %d", rowsAffected, targetID)
	return nil
}
