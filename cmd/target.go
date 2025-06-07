package cmd

import (
	"database/sql"
	"encoding/json" // ADDED import for json
	"errors"
	"fmt"
	"os"
	"path/filepath" // ADDED import for filepath
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"toolkit/config"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// --- State File Structure ---
type AppState struct {
	CurrentTargetID int64 `json:"current_target_id,omitempty"`
}

// --- Helper Functions ---

// slugify creates a URL-friendly slug from a string.
func slugify(s string) string {
	slug := strings.ToLower(s)
	reg := regexp.MustCompile("[^a-z0-9]+")
	slug = reg.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return uuid.New().String()[:8]
	}
	return slug
}

// isValidURL checks if the string is a somewhat valid URL.
func isValidURL(toTest string) bool {
	return strings.HasPrefix(toTest, "http://") || strings.HasPrefix(toTest, "https://") || (strings.Contains(toTest, ".") && !strings.ContainsAny(toTest, " \t\n"))
}

// --- State File Helpers ---
func getStateFilePath() string {
	return config.GetDefaultConfigPaths().StateFilePath
}

func readState() (*AppState, error) {
	stateFilePath := getStateFilePath()
	state := &AppState{}

	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		logger.Error("Failed to read state file '%s': %v", stateFilePath, err)
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	if len(data) == 0 {
		return state, nil
	}

	err = json.Unmarshal(data, state) // Uses json
	if err != nil {
		logger.Error("Failed to unmarshal state file '%s': %v", stateFilePath, err)
		return &AppState{}, fmt.Errorf("failed to parse state file: %w", err)
	}
	return state, nil
}

func writeState(state *AppState) error {
	stateFilePath := getStateFilePath()
	data, err := json.MarshalIndent(state, "", "  ") // Uses json
	if err != nil {
		logger.Error("Failed to marshal state: %v", err)
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(stateFilePath), 0750); err != nil { // Uses filepath
		logger.Error("Failed to create directory for state file '%s': %v", stateFilePath, err)
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	err = os.WriteFile(stateFilePath, data, 0640)
	if err != nil {
		logger.Error("Failed to write state file '%s': %v", stateFilePath, err)
		return fmt.Errorf("failed to write state file: %w", err)
	}
	logger.Debug("State file updated: %s", stateFilePath)
	return nil
}

// Helper to get target ID from identifier (ID, slug, or codename+platformID)
func getTargetIDByIdentifier(identifier string, platformID int64) (int64, error) {
	targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
	if parseErr == nil {
		var exists bool
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", targetID).Scan(&exists)
		if err != nil {
			return 0, fmt.Errorf("error verifying target ID %d: %w", targetID, err)
		}
		if !exists {
			return 0, fmt.Errorf("target with ID %d not found", targetID)
		}
		logger.Debug("Found target by ID: %d", targetID)
		return targetID, nil
	}

	var idBySlug int64
	err := database.DB.QueryRow("SELECT id FROM targets WHERE slug = ?", identifier).Scan(&idBySlug)
	if err == nil {
		logger.Debug("Found target by slug '%s': ID %d", identifier, idBySlug)
		return idBySlug, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("error finding target by slug '%s': %w", identifier, err)
	}

	if platformID == 0 {
		return 0, fmt.Errorf("identifier '%s' is not a valid ID or slug, and --platform-id was not provided to check by codename", identifier)
	}

	var idByCodename int64
	err = database.DB.QueryRow("SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)", platformID, identifier).Scan(&idByCodename)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("target with codename '%s' not found on platform ID %d", identifier, platformID)
		}
		return 0, fmt.Errorf("error finding target by codename '%s' on platform ID %d: %w", identifier, platformID, err)
	}
	logger.Debug("Found target by codename '%s' on platform %d: ID %d", identifier, platformID, idByCodename)
	return idByCodename, nil
}

// --- Command Definitions ---

var (
	targetListPlatformID int64
	targetPlatformID     int64
	targetCodename       string
	targetLink           string
	targetNotes          string
	targetSlug           string
	deleteByCodename     bool
)

var targetCmd = &cobra.Command{
	Use:     "target",
	Short:   "Manage targets within platforms",
	Long:    `Allows you to list, add, get, update, delete, or set the current target.`,
	Aliases: []string{"t"},
}

// --- List Command ---

var targetListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List stored targets",
	Long:    `Retrieves and displays targets, optionally filtered by platform ID.`,
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'target list' command")

		query := "SELECT id, platform_id, slug, codename, link, notes FROM targets"
		dbArgs := []interface{}{}

		if targetListPlatformID != 0 {
			logger.Info("Filtering targets by platform_id: %d", targetListPlatformID)
			query += " WHERE platform_id = ?"
			dbArgs = append(dbArgs, targetListPlatformID)
		}
		query += " ORDER BY platform_id ASC, codename ASC"

		rows, err := database.DB.Query(query, dbArgs...)
		if err != nil {
			logger.Error("Failed to query targets: %v", err)
			fmt.Fprintln(os.Stderr, "Error retrieving targets from database.")
			os.Exit(1)
		}
		defer rows.Close()

		targets := []models.Target{}
		for rows.Next() {
			var t models.Target
			var slug sql.NullString
			var notes sql.NullString
			if err := rows.Scan(&t.ID, &t.PlatformID, &slug, &t.Codename, &t.Link, &notes); err != nil {
				logger.Error("Failed to scan target row: %v", err)
				fmt.Fprintln(os.Stderr, "Error reading target data from database.")
				os.Exit(1)
			}
			if slug.Valid {
				t.Slug = slug.String
			}
			if notes.Valid {
				t.Notes = notes.String
			}
			targets = append(targets, t)
		}
		if err = rows.Err(); err != nil {
			logger.Error("Error iterating target rows: %v", err)
			fmt.Fprintln(os.Stderr, "Error iterating target results.")
			os.Exit(1)
		}

		if len(targets) == 0 {
			if targetListPlatformID != 0 {
				fmt.Printf("No targets found for platform ID %d.\n", targetListPlatformID)
			} else {
				fmt.Println("No targets found in the database.")
			}
			return
		}

		if targetListPlatformID != 0 {
			fmt.Printf("Targets for Platform ID %d:\n", targetListPlatformID)
		} else {
			fmt.Println("All Stored Targets:")
		}
		writer := new(tabwriter.Writer)
		writer.Init(os.Stdout, 0, 8, 1, '\t', 0)

		fmt.Fprintln(writer, "ID\tPLATFORM_ID\tSLUG\tCODENAME\tLINK\tNOTES")
		fmt.Fprintln(writer, "--\t-----------\t----\t--------\t----\t-----")

		for _, t := range targets {
			fmt.Fprintf(writer, "%d\t%d\t%s\t%s\t%s\t%s\n",
				t.ID,
				t.PlatformID,
				t.Slug,
				t.Codename,
				t.Link,
				t.Notes,
			)
		}
		writer.Flush()
		logger.Info("Successfully listed %d targets", len(targets))
	},
}

// --- Add Command ---

var targetAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new target",
	Long:  `Adds a new target to a specified platform in the database. Requires platform ID, codename, link, and a unique slug.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'target add' command")

		if targetPlatformID == 0 {
			fmt.Fprintln(os.Stderr, "Error: --platform-id is required.")
			os.Exit(1)
		}
		targetCodename = strings.TrimSpace(targetCodename)
		if targetCodename == "" {
			fmt.Fprintln(os.Stderr, "Error: --codename is required.")
			os.Exit(1)
		}
		targetLink = strings.TrimSpace(targetLink)
		if targetLink == "" {
			fmt.Fprintln(os.Stderr, "Error: --link is required.")
			os.Exit(1)
		}
		if !isValidURL(targetLink) {
			fmt.Fprintln(os.Stderr, "Error: --link must be a valid URL.")
			os.Exit(1)
		}
		targetSlug = strings.TrimSpace(targetSlug)
		if targetSlug == "" {
			fmt.Fprintln(os.Stderr, "Error: --slug is required.")
			os.Exit(1)
		}
		if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(targetSlug) {
			fmt.Fprintln(os.Stderr, "Error: --slug can only contain lowercase letters, numbers, and hyphens.")
			os.Exit(1)
		}

		var platformExists bool
		err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM platforms WHERE id = ?)", targetPlatformID).Scan(&platformExists)
		if err != nil {
			logger.Error("target add: Error checking platform existence for PlatformID %d: %v", targetPlatformID, err)
			fmt.Fprintln(os.Stderr, "Error verifying platform ID.")
			os.Exit(1)
		}
		if !platformExists {
			fmt.Fprintf(os.Stderr, "Error: Platform with ID %d does not exist.\n", targetPlatformID)
			os.Exit(1)
		}

		var existingTargetID int64
		err = database.DB.QueryRow("SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)", targetPlatformID, targetCodename).Scan(&existingTargetID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Error("target add: Error checking for existing target codename '%s' under platform %d: %v", targetCodename, targetPlatformID, err)
			fmt.Fprintln(os.Stderr, "Error checking for duplicate target.")
			os.Exit(1)
		}
		if err == nil {
			fmt.Fprintf(os.Stderr, "Error: Target with codename '%s' already exists for platform ID %d.\n", targetCodename, targetPlatformID)
			os.Exit(1)
		}

		var existingSlugID int64
		err = database.DB.QueryRow("SELECT id FROM targets WHERE slug = ?", targetSlug).Scan(&existingSlugID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Error("target add: Error checking for existing slug '%s': %v", targetSlug, err)
			fmt.Fprintln(os.Stderr, "Error checking for existing slug.")
			os.Exit(1)
		}
		if err == nil {
			fmt.Fprintf(os.Stderr, "Error: Target with slug '%s' already exists (ID: %d).\n", targetSlug, existingSlugID)
			os.Exit(1)
		}

		stmt, err := database.DB.Prepare(`INSERT INTO targets
            (platform_id, slug, codename, link, notes)
            VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			logger.Error("target add: Error preparing target insert statement for '%s': %v", targetCodename, err)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		res, err := stmt.Exec(targetPlatformID, targetSlug, targetCodename, targetLink, targetNotes)
		if err != nil {
			logger.Error("target add: Error inserting target '%s': %v", targetCodename, err)
			if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
				fmt.Fprintln(os.Stderr, "Error: Target insert failed due to unique constraint (platform+codename or slug).")
			} else {
				fmt.Fprintln(os.Stderr, "Error inserting target into database.")
			}
			os.Exit(1)
		}

		id, err := res.LastInsertId()
		if err != nil {
			logger.Error("target add: Error getting last insert ID for target '%s': %v", targetCodename, err)
			fmt.Printf("Target '%s' added successfully, but could not retrieve new ID.\n", targetCodename)
		} else {
			fmt.Printf("Successfully added target:\n")
			fmt.Printf("  ID:          %d\n", id)
			fmt.Printf("  Platform ID: %d\n", targetPlatformID)
			fmt.Printf("  Codename:    %s\n", targetCodename)
			fmt.Printf("  Slug:        %s\n", targetSlug)
			fmt.Printf("  Link:        %s\n", targetLink)
			fmt.Printf("  Notes:       %s\n", targetNotes)
			logger.Info("Target added via CLI: ID %d, Codename '%s', PlatformID %d, Slug '%s'", id, targetCodename, targetPlatformID, targetSlug)
		}
	},
}

// --- Get Command ---

var targetGetCmd = &cobra.Command{
	Use:   "get [id|slug]",
	Short: "Get details for a specific target",
	Long:  `Retrieves and displays details for a single target, identified by its numeric ID or unique slug.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		logger.Info("Executing 'target get' command for identifier: %s", identifier)

		var t models.Target
		var query string
		var scanArgs []interface{}
		var slug sql.NullString
		var notes sql.NullString

		targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
		if parseErr == nil {
			query = `SELECT id, platform_id, slug, codename, link, notes FROM targets WHERE id = ?`
			scanArgs = append(scanArgs, &t.ID, &t.PlatformID, &slug, &t.Codename, &t.Link, &notes)
			err := database.DB.QueryRow(query, targetID).Scan(scanArgs...)
			if handleGetTargetError(err, fmt.Sprintf("ID %d", targetID)) {
				os.Exit(1)
			}
		} else {
			query = `SELECT id, platform_id, slug, codename, link, notes FROM targets WHERE slug = ?`
			scanArgs = append(scanArgs, &t.ID, &t.PlatformID, &slug, &t.Codename, &t.Link, &notes)
			err := database.DB.QueryRow(query, identifier).Scan(scanArgs...)
			if handleGetTargetError(err, fmt.Sprintf("slug '%s'", identifier)) {
				os.Exit(1)
			}
		}

		if slug.Valid {
			t.Slug = slug.String
		}
		if notes.Valid {
			t.Notes = notes.String
		}

		scopeRules, err := getScopeRulesForTarget(t.ID)
		if err != nil {
			logger.Error("target get: Failed to fetch scope rules for target ID %d: %v", t.ID, err)
			fmt.Fprintln(os.Stderr, "Warning: could not fetch scope rules.")
		}

		fmt.Printf("Target Details (Identifier: %s):\n", identifier)
		fmt.Printf("  ID:          %d\n", t.ID)
		fmt.Printf("  Platform ID: %d\n", t.PlatformID)
		fmt.Printf("  Codename:    %s\n", t.Codename)
		fmt.Printf("  Slug:        %s\n", t.Slug)
		fmt.Printf("  Link:        %s\n", t.Link)
		fmt.Printf("  Notes:       %s\n", t.Notes)
		fmt.Println("  Scope Rules:")
		if len(scopeRules) == 0 {
			fmt.Println("    (No scope rules defined)")
		} else {
			writer := new(tabwriter.Writer)
			writer.Init(os.Stdout, 4, 8, 1, '\t', 0)
			fmt.Fprintln(writer, "    ID\tTYPE\tIN_SCOPE\tPATTERN\tDESCRIPTION")
			fmt.Fprintln(writer, "    --\t----\t--------\t-------\t-----------")
			for _, sr := range scopeRules {
				inScopeStr := "OUT"
				if sr.IsInScope {
					inScopeStr = "IN"
				}
				fmt.Fprintf(writer, "    %d\t%s\t%s\t%s\t%s\n", sr.ID, sr.ItemType, inScopeStr, sr.Pattern, sr.Description)
			}
			writer.Flush()
		}
		logger.Info("Successfully retrieved target %s", identifier)
	},
}

// Helper for handling errors in targetGetCmd
func handleGetTargetError(err error, identifier string) bool {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fmt.Fprintf(os.Stderr, "Error: Target with %s not found.\n", identifier)
		} else {
			logger.Error("target get: Error querying target %s: %v", identifier, err)
			fmt.Fprintf(os.Stderr, "Error retrieving target %s from database.\n", identifier)
		}
		return true
	}
	return false
}

// Helper to get scope rules for target get command
func getScopeRulesForTarget(targetID int64) ([]models.ScopeRule, error) {
	rows, err := database.DB.Query(`SELECT id, target_id, item_type, pattern, is_in_scope, is_wildcard, description 
                                    FROM scope_rules WHERE target_id = ? ORDER BY id ASC`, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopeRules := []models.ScopeRule{}
	for rows.Next() {
		var sr models.ScopeRule
		var description sql.NullString
		if err := rows.Scan(&sr.ID, &sr.TargetID, &sr.ItemType, &sr.Pattern, &sr.IsInScope, &sr.IsWildcard, &description); err != nil {
			return nil, err
		}
		if description.Valid {
			sr.Description = description.String
		}
		scopeRules = append(scopeRules, sr)
	}
	return scopeRules, rows.Err()
}


// --- Update Command ---

var targetUpdateCmd = &cobra.Command{
	Use:   "update [id|slug]",
	Short: "Update details for a specific target",
	Long:  `Updates details (like codename, link, notes, slug) for a single target, identified by its numeric ID or current unique slug.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		logger.Info("Executing 'target update' command for identifier: %s", identifier)

		if !cmd.Flags().Changed("codename") && !cmd.Flags().Changed("link") && !cmd.Flags().Changed("notes") && !cmd.Flags().Changed("platform-id") && !cmd.Flags().Changed("slug") {
			fmt.Fprintln(os.Stderr, "Error: At least one field flag (--codename, --link, --notes, --platform-id, --slug) must be provided to update.")
			os.Exit(1)
		}

		if cmd.Flags().Changed("link") {
			targetLink = strings.TrimSpace(targetLink)
			if !isValidURL(targetLink) {
				fmt.Fprintln(os.Stderr, "Error: --link must be a valid URL.")
				os.Exit(1)
			}
		}
		if cmd.Flags().Changed("codename") {
			targetCodename = strings.TrimSpace(targetCodename)
			if targetCodename == "" {
				fmt.Fprintln(os.Stderr, "Error: --codename cannot be empty.")
				os.Exit(1)
			}
		}
		if cmd.Flags().Changed("slug") {
			targetSlug = strings.TrimSpace(targetSlug)
			if targetSlug == "" {
				fmt.Fprintln(os.Stderr, "Error: --slug cannot be empty.")
				os.Exit(1)
			}
			if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(targetSlug) {
				fmt.Fprintln(os.Stderr, "Error: --slug can only contain lowercase letters, numbers, and hyphens.")
				os.Exit(1)
			}
		}

		if cmd.Flags().Changed("platform-id") {
			if targetPlatformID == 0 {
				fmt.Fprintln(os.Stderr, "Error: --platform-id cannot be zero.")
				os.Exit(1)
			}
			var platformExists bool
			err := database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM platforms WHERE id = ?)", targetPlatformID).Scan(&platformExists)
			if err != nil {
				logger.Error("target update: Error checking platform existence for PlatformID %d: %v", targetPlatformID, err)
				fmt.Fprintln(os.Stderr, "Error verifying platform ID.")
				os.Exit(1)
			}
			if !platformExists {
				fmt.Fprintf(os.Stderr, "Error: Platform with ID %d does not exist.\n", targetPlatformID)
				os.Exit(1)
			}
		}

		// Check for potential UNIQUE constraint violations before attempting update
		if cmd.Flags().Changed("codename") || cmd.Flags().Changed("platform-id") {
			checkPlatformID := targetPlatformID
			newCodename := targetCodename
			if !cmd.Flags().Changed("platform-id") || !cmd.Flags().Changed("codename") {
				var currentTarget models.Target
				var currentQuery string
				var currentArgs []interface{}
				targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
				if parseErr == nil {
					currentQuery = "SELECT platform_id, codename FROM targets WHERE id = ?"
					currentArgs = append(currentArgs, targetID)
				} else {
					currentQuery = "SELECT platform_id, codename FROM targets WHERE slug = ?"
					currentArgs = append(currentArgs, identifier)
				}
				err := database.DB.QueryRow(currentQuery, currentArgs...).Scan(&currentTarget.PlatformID, &currentTarget.Codename)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					logger.Error("target update: Error fetching current target details for conflict check: %v", err)
					fmt.Fprintln(os.Stderr, "Error checking for potential conflicts.")
					os.Exit(1)
				} else if err == nil {
					if !cmd.Flags().Changed("platform-id") { checkPlatformID = currentTarget.PlatformID }
					if !cmd.Flags().Changed("codename") { newCodename = currentTarget.Codename }
				}
			}
			var conflictID int64
			var conflictQuery string
			var conflictArgs []interface{}
			targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
			if parseErr == nil {
				conflictQuery = "SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?) AND id != ?"
				conflictArgs = append(conflictArgs, checkPlatformID, newCodename, targetID)
			} else {
				conflictQuery = "SELECT id FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?) AND slug != ?"
				conflictArgs = append(conflictArgs, checkPlatformID, newCodename, identifier)
			}
			err := database.DB.QueryRow(conflictQuery, conflictArgs...).Scan(&conflictID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				logger.Error("target update: Error checking for conflicting target name '%s' on platform %d: %v", newCodename, checkPlatformID, err)
				fmt.Fprintln(os.Stderr, "Error checking for conflicting target name.")
				os.Exit(1)
			}
			if err == nil {
				fmt.Fprintf(os.Stderr, "Error: Another target (ID: %d) already exists with codename '%s' on platform %d.\n", conflictID, newCodename, checkPlatformID)
				os.Exit(1)
			}
		}
		if cmd.Flags().Changed("slug") {
			var conflictID int64
			var conflictQuery string
			var conflictArgs []interface{}
			targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
			if parseErr == nil {
				conflictQuery = "SELECT id FROM targets WHERE slug = ? AND id != ?"
				conflictArgs = append(conflictArgs, targetSlug, targetID)
			} else {
				conflictQuery = "SELECT id FROM targets WHERE slug = ? AND slug != ?"
				conflictArgs = append(conflictArgs, targetSlug, identifier)
			}
			err := database.DB.QueryRow(conflictQuery, conflictArgs...).Scan(&conflictID)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				logger.Error("target update: Error checking for conflicting slug '%s': %v", targetSlug, err)
				fmt.Fprintln(os.Stderr, "Error checking for conflicting slug.")
				os.Exit(1)
			}
			if err == nil {
				fmt.Fprintf(os.Stderr, "Error: Another target (ID: %d) already exists with the slug '%s'.\n", conflictID, targetSlug)
				os.Exit(1)
			}
		}

		// Prepare update query
		query := "UPDATE targets SET "
		updateArgs := []interface{}{}
		updates := []string{}

		if cmd.Flags().Changed("codename") {
			updates = append(updates, "codename = ?")
			updateArgs = append(updateArgs, targetCodename)
		}
		if cmd.Flags().Changed("link") {
			updates = append(updates, "link = ?")
			updateArgs = append(updateArgs, targetLink)
		}
		if cmd.Flags().Changed("notes") {
			updates = append(updates, "notes = ?")
			updateArgs = append(updateArgs, targetNotes)
		}
		if cmd.Flags().Changed("platform-id") {
			updates = append(updates, "platform_id = ?")
			updateArgs = append(updateArgs, targetPlatformID)
		}
		if cmd.Flags().Changed("slug") {
			updates = append(updates, "slug = ?")
			updateArgs = append(updateArgs, targetSlug)
		}

		query += strings.Join(updates, ", ")

		var whereClause string
		targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
		if parseErr == nil {
			whereClause = " WHERE id = ?"
			updateArgs = append(updateArgs, targetID)
		} else {
			whereClause = " WHERE slug = ?"
			updateArgs = append(updateArgs, identifier)
		}
		query += whereClause

		logger.Debug("Update query: %s", query)
		logger.Debug("Update args: %v", updateArgs)

		stmt, err := database.DB.Prepare(query)
		if err != nil {
			logger.Error("target update: Error preparing update statement for '%s': %v", identifier, err)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		result, err := stmt.Exec(updateArgs...)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
				logger.Error("target update: Update for '%s' failed due to UNIQUE constraint: %v", identifier, err)
				fmt.Fprintln(os.Stderr, "Error: Update violates a unique constraint (platform+codename or slug).")
			} else {
				logger.Error("target update: Error executing update for '%s': %v", identifier, err)
				fmt.Fprintln(os.Stderr, "Error updating target in database.")
			}
			os.Exit(1)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			fmt.Fprintf(os.Stderr, "Error: Target '%s' not found.\n", identifier)
			os.Exit(1)
		}

		fmt.Printf("Successfully updated target '%s'.\n", identifier)
		logger.Info("Target updated via CLI: Identifier '%s'", identifier)

	},
}


// --- Delete Command ---

var targetDeleteCmd = &cobra.Command{
	Use:   "delete [id|slug|codename]",
	Short: "Delete a specific target",
	Long:  `Deletes a single target, identified by its numeric ID, unique slug, or codename (requires --platform-id if using codename).`,
	Args:  cobra.ExactArgs(1),
	Aliases: []string{"del", "rm"},
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		logger.Info("Executing 'target delete' command for identifier: %s", identifier)

		var result sql.Result
		var err error
		var deletedBy string
		var query string
		var dbArgs []interface{}

		if deleteByCodename {
			if targetPlatformID == 0 {
				fmt.Fprintln(os.Stderr, "Error: --platform-id is required when deleting by codename.")
				os.Exit(1)
			}
			deletedBy = fmt.Sprintf("codename '%s' on platform %d", identifier, targetPlatformID)
			query = "DELETE FROM targets WHERE platform_id = ? AND LOWER(codename) = LOWER(?)"
			dbArgs = append(dbArgs, targetPlatformID, identifier)
		} else {
			targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
			if parseErr == nil {
				deletedBy = fmt.Sprintf("ID %d", targetID)
				query = "DELETE FROM targets WHERE id = ?"
				dbArgs = append(dbArgs, targetID)
			} else {
				deletedBy = fmt.Sprintf("slug '%s'", identifier)
				query = "DELETE FROM targets WHERE slug = ?"
				dbArgs = append(dbArgs, identifier)
			}
		}

		logger.Info("Attempting to delete target by %s", deletedBy)

		stmt, errPrep := database.DB.Prepare(query)
		if errPrep != nil {
			logger.Error("target delete: Error preparing delete statement for target %s: %v", deletedBy, errPrep)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		result, err = stmt.Exec(dbArgs...)
		if err != nil {
			logger.Error("target delete: Error executing delete statement for target %s: %v", deletedBy, err)
			fmt.Fprintln(os.Stderr, "Error deleting target from database.")
			os.Exit(1)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			logger.Error("target delete: Error getting rows affected for target %s: %v", deletedBy, err)
			fmt.Printf("Successfully deleted target %s (unable to verify rows affected).\n", deletedBy)
			logger.Info("Target deleted via CLI: %s", deletedBy)
			return
		}

		if rowsAffected == 0 {
			fmt.Fprintf(os.Stderr, "Error: Target %s not found.\n", deletedBy)
			os.Exit(1)
		}

		fmt.Printf("Successfully deleted target %s.\n", deletedBy)
		logger.Info("Target deleted via CLI: %s", deletedBy)
	},
}

// --- Set Current Command ---

var targetSetCurrentCmd = &cobra.Command{
	Use:   "set-current [id|slug|codename]",
	Short: "Set the currently active target",
	Long: `Sets the specified target as the active target in the state file (~/.config/toolkit/state.json).
Identify the target by its numeric ID, unique slug, or codename.
If using codename, the --platform-id flag is required.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		// Note: targetPlatformID flag is reused here from the update/add commands
		logger.Info("Executing 'target set-current' for identifier: %s (Platform ID if codename: %d)", identifier, targetPlatformID)

		targetID, err := getTargetIDByIdentifier(identifier, targetPlatformID)
		if err != nil {
			logger.Error("target set-current: %v", err)
			fmt.Fprintf(os.Stderr, "Error finding target: %v\n", err)
			if strings.Contains(err.Error(), "--platform-id was not provided") {
				fmt.Fprintln(os.Stderr, "Hint: If using a codename, you must also provide the --platform-id flag.")
			}
			os.Exit(1)
		}

		state, err := readState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not read existing state file: %v\n", err)
		}

		state.CurrentTargetID = targetID

		err = writeState(state)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing state file: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Successfully set current target to ID: %d (Identifier: %s)\n", targetID, identifier)
		logger.Info("Current target set to ID %d", targetID)
	},
}

// --- Current Command ---

var targetCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the currently active target",
	Long:  `Displays the details of the target currently set as active in the state file.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'target current' command")

		state, err := readState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading state file: %v\n", err)
			os.Exit(1)
		}

		if state.CurrentTargetID == 0 {
			fmt.Println("No current target is set.")
			fmt.Println("Use 'toolkit target set-current [id|slug|codename] [--platform-id ID_IF_CODENAME]' to set one.")
			return
		}

		logger.Info("Current target ID from state file: %d", state.CurrentTargetID)
		targetGetCmd.Run(cmd, []string{strconv.FormatInt(state.CurrentTargetID, 10)})
	},
}


// --- Init Function ---

func init() {
	// Add list command flags
	targetListCmd.Flags().Int64VarP(&targetListPlatformID, "platform-id", "p", 0, "Filter targets by specific platform ID")

	// Add add command flags
	targetAddCmd.Flags().Int64VarP(&targetPlatformID, "platform-id", "p", 0, "Platform ID for the new target (required)")
	targetAddCmd.Flags().StringVarP(&targetCodename, "codename", "c", "", "Codename for the new target (required)")
	targetAddCmd.Flags().StringVarP(&targetLink, "link", "l", "", "Primary link/URL for the new target (required)")
	targetAddCmd.Flags().StringVarP(&targetSlug, "slug", "s", "", "Unique slug for the new target (required, lowercase, numbers, hyphens only)")
	targetAddCmd.Flags().StringVarP(&targetNotes, "notes", "n", "", "Optional notes for the new target")
	targetAddCmd.MarkFlagRequired("platform-id")
	targetAddCmd.MarkFlagRequired("codename")
	targetAddCmd.MarkFlagRequired("link")
	targetAddCmd.MarkFlagRequired("slug")

	// Add update command flags
	targetUpdateCmd.Flags().Int64VarP(&targetPlatformID, "platform-id", "p", 0, "Update target's platform ID")
	targetUpdateCmd.Flags().StringVarP(&targetCodename, "codename", "c", "", "Update target's codename")
	targetUpdateCmd.Flags().StringVarP(&targetLink, "link", "l", "", "Update target's link/URL")
	targetUpdateCmd.Flags().StringVarP(&targetSlug, "slug", "s", "", "Update target's unique slug (lowercase, numbers, hyphens only)")
	targetUpdateCmd.Flags().StringVarP(&targetNotes, "notes", "n", "", "Update target's notes")

	// Add delete command flags
	targetDeleteCmd.Flags().BoolVar(&deleteByCodename, "by-codename", false, "Delete by codename instead of ID/slug (requires --platform-id)")
	targetDeleteCmd.Flags().Int64VarP(&targetPlatformID, "platform-id", "p", 0, "Platform ID (required when deleting --by-codename)")

	// Add set-current command flags
	targetSetCurrentCmd.Flags().Int64VarP(&targetPlatformID, "platform-id", "p", 0, "Platform ID (required when setting current target by codename)")


	// Add subcommands to the base target command
	targetCmd.AddCommand(targetListCmd)
	targetCmd.AddCommand(targetAddCmd)
	targetCmd.AddCommand(targetGetCmd)
	targetCmd.AddCommand(targetUpdateCmd)
	targetCmd.AddCommand(targetDeleteCmd)
	targetCmd.AddCommand(targetSetCurrentCmd)
	targetCmd.AddCommand(targetCurrentCmd)

	// Add the base target command to the root command
	rootCmd.AddCommand(targetCmd)
}