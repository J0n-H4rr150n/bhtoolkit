package cmd

import (
	"database/sql" // Added for sql.ErrNoRows and sql.NullString
	"errors"       // Added for error checking
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter" // For aligned table output
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/spf13/cobra"
)

// --- Flags ---
var (
	// Flags for update/add
	platformName string // Used by add and update commands
)

// --- Base Command ---

// platformCmd represents the base command for platform operations
var platformCmd = &cobra.Command{
	Use:     "platform",
	Short:   "Manage bug bounty platforms",
	Long:    `Allows you to list, add, get, update, or delete bug bounty platforms stored in the toolkit database.`,
	Aliases: []string{"p", "plat"},
}

// --- List Command ---

var platformListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List all stored platforms",
	Long:    `Retrieves and displays all the bug bounty platforms currently stored in the database.`,
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'platform list' command")
		rows, err := database.DB.Query("SELECT id, name FROM platforms ORDER BY name ASC")
		if err != nil {
			logger.Error("Failed to query platforms: %v", err)
			fmt.Fprintln(os.Stderr, "Error retrieving platforms from database.")
			os.Exit(1)
		}
		defer rows.Close()

		platforms := []models.Platform{}
		for rows.Next() {
			var p models.Platform
			if err := rows.Scan(&p.ID, &p.Name); err != nil {
				logger.Error("Failed to scan platform row: %v", err)
				fmt.Fprintln(os.Stderr, "Error reading platform data from database.")
				os.Exit(1)
			}
			platforms = append(platforms, p)
		}
		if err = rows.Err(); err != nil {
			logger.Error("Error iterating platform rows: %v", err)
			fmt.Fprintln(os.Stderr, "Error iterating platform results.")
			os.Exit(1)
		}

		if len(platforms) == 0 {
			fmt.Println("No platforms found in the database.")
			return
		}

		fmt.Println("Stored Platforms:")
		writer := new(tabwriter.Writer)
		writer.Init(os.Stdout, 0, 8, 1, '\t', 0)
		fmt.Fprintln(writer, "ID\tNAME")
		fmt.Fprintln(writer, "--\t----")
		for _, p := range platforms {
			fmt.Fprintf(writer, "%d\t%s\n", p.ID, p.Name)
		}
		writer.Flush()
		logger.Info("Successfully listed %d platforms", len(platforms))
	},
}

// --- Add Command ---

var platformAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new platform",
	Long:  `Adds a new bug bounty platform to the database. Platform name must be unique.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'platform add' command")

		platformName = strings.TrimSpace(platformName)
		if platformName == "" {
			fmt.Fprintln(os.Stderr, "Error: --name flag is required and cannot be empty.")
			os.Exit(1)
		}

		// Check if platform already exists (case-insensitive check for user convenience)
		var existingID int64
		err := database.DB.QueryRow("SELECT id FROM platforms WHERE LOWER(name) = LOWER(?)", platformName).Scan(&existingID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Error("platform add: Error checking for existing platform '%s': %v", platformName, err)
			fmt.Fprintln(os.Stderr, "Error checking for existing platform.")
			os.Exit(1)
		}
		if err == nil { // Found existing
			fmt.Fprintf(os.Stderr, "Error: Platform '%s' already exists with ID %d.\n", platformName, existingID)
			os.Exit(1)
		}

		// Insert new platform
		stmt, err := database.DB.Prepare("INSERT INTO platforms(name) VALUES(?)")
		if err != nil {
			logger.Error("platform add: Error preparing statement for platform '%s': %v", platformName, err)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		res, err := stmt.Exec(platformName)
		if err != nil {
			// Catch potential DB-level UNIQUE constraint violation
			if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
				fmt.Fprintf(os.Stderr, "Error: Platform name '%s' already exists (database constraint).\n", platformName)
			} else {
				logger.Error("platform add: Error inserting platform '%s': %v", platformName, err)
				fmt.Fprintln(os.Stderr, "Error inserting platform into database.")
			}
			os.Exit(1)
		}

		id, err := res.LastInsertId()
		if err != nil {
			logger.Error("platform add: Error getting last insert ID for platform '%s': %v", platformName, err)
			fmt.Printf("Platform '%s' added successfully, but could not retrieve new ID.\n", platformName)
		} else {
			fmt.Printf("Successfully added platform: ID %d, Name '%s'\n", id, platformName)
			logger.Info("Platform added via CLI: ID %d, Name '%s'", id, platformName)
		}
	},
}

// --- Get Command ---

var platformGetCmd = &cobra.Command{
	Use:   "get [id|name]",
	Short: "Get details for a specific platform",
	Long:  `Retrieves and displays details for a single platform, identified by its numeric ID or unique name.`,
	Args:  cobra.ExactArgs(1), // Requires exactly one argument: the id or name
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		logger.Info("Executing 'platform get' command for identifier: %s", identifier)

		var p models.Platform
		var query string
		var scanErr error

		// Try parsing as ID first
		platformID, parseErr := strconv.ParseInt(identifier, 10, 64)
		if parseErr == nil {
			// It's a numeric ID
			query = `SELECT id, name FROM platforms WHERE id = ?`
			scanErr = database.DB.QueryRow(query, platformID).Scan(&p.ID, &p.Name)
		} else {
			// Not numeric, assume it's a name (case-insensitive search)
			query = `SELECT id, name FROM platforms WHERE LOWER(name) = LOWER(?)`
			scanErr = database.DB.QueryRow(query, identifier).Scan(&p.ID, &p.Name)
		}

		if scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				fmt.Fprintf(os.Stderr, "Error: Platform '%s' not found.\n", identifier)
			} else {
				logger.Error("platform get: Error querying platform '%s': %v", identifier, scanErr)
				fmt.Fprintf(os.Stderr, "Error retrieving platform '%s' from database.\n", identifier)
			}
			os.Exit(1)
		}

		// Print Platform Details
		fmt.Printf("Platform Details (Identifier: %s):\n", identifier)
		fmt.Printf("  ID:   %d\n", p.ID)
		fmt.Printf("  Name: %s\n", p.Name)
		logger.Info("Successfully retrieved platform %s", identifier)
	},
}

// --- Update Command ---

var platformUpdateCmd = &cobra.Command{
	Use:   "update [id|name]",
	Short: "Update the name of a specific platform",
	Long:  `Updates the name of a single platform, identified by its numeric ID or current unique name. Requires the --name flag for the new name.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		logger.Info("Executing 'platform update' command for identifier: %s", identifier)

		// Validate required --name flag for the new name
		platformName = strings.TrimSpace(platformName)
		if platformName == "" {
			fmt.Fprintln(os.Stderr, "Error: --name flag with the new name is required for update.")
			os.Exit(1)
		}

		// Check if the *new* name already exists (excluding the one we might be updating)
		var existingID int64
		var checkQuery string
		var checkArgs []interface{}

		targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
		if parseErr == nil { // Updating by ID
			checkQuery = "SELECT id FROM platforms WHERE LOWER(name) = LOWER(?) AND id != ?"
			checkArgs = append(checkArgs, platformName, targetID)
		} else { // Updating by current name
			checkQuery = "SELECT id FROM platforms WHERE LOWER(name) = LOWER(?) AND LOWER(name) != LOWER(?)"
			checkArgs = append(checkArgs, platformName, identifier)
		}

		err := database.DB.QueryRow(checkQuery, checkArgs...).Scan(&existingID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Error("platform update: Error checking for conflicting new name '%s': %v", platformName, err)
			fmt.Fprintln(os.Stderr, "Error checking for conflicting name.")
			os.Exit(1)
		}
		if err == nil { // Found conflicting name
			fmt.Fprintf(os.Stderr, "Error: Another platform already exists with the name '%s'.\n", platformName)
			os.Exit(1)
		}

		// Prepare update query
		query := "UPDATE platforms SET name = ?"
		updateArgs := []interface{}{platformName}
		var whereClause string

		if parseErr == nil { // Updating by ID
			whereClause = " WHERE id = ?"
			updateArgs = append(updateArgs, targetID)
		} else { // Updating by current name
			whereClause = " WHERE LOWER(name) = LOWER(?)"
			updateArgs = append(updateArgs, identifier)
		}
		query += whereClause

		logger.Debug("Update query: %s", query)
		logger.Debug("Update args: %v", updateArgs)

		stmt, err := database.DB.Prepare(query)
		if err != nil {
			logger.Error("platform update: Error preparing update statement for '%s': %v", identifier, err)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		result, err := stmt.Exec(updateArgs...)
		if err != nil {
			// This might catch the DB-level UNIQUE constraint if the pre-check failed
			if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
				logger.Error("platform update: Update for '%s' failed due to UNIQUE constraint: %v", identifier, err)
				fmt.Fprintf(os.Stderr, "Error: Update failed because the name '%s' already exists.\n", platformName)
			} else {
				logger.Error("platform update: Error executing update for '%s': %v", identifier, err)
				fmt.Fprintln(os.Stderr, "Error updating platform in database.")
			}
			os.Exit(1)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			fmt.Fprintf(os.Stderr, "Error: Platform '%s' not found.\n", identifier)
			os.Exit(1)
		}

		fmt.Printf("Successfully updated platform '%s' to name '%s'.\n", identifier, platformName)
		logger.Info("Platform updated via CLI: Identifier '%s', New Name '%s'", identifier, platformName)
	},
}

// --- Delete Command ---

var platformDeleteCmd = &cobra.Command{
	Use:     "delete [id|name]",
	Short:   "Delete a specific platform",
	Long:    `Deletes a single platform, identified by its numeric ID or unique name. WARNING: This will also delete all associated targets and scope rules due to database constraints (ON DELETE CASCADE).`,
	Args:    cobra.ExactArgs(1),
	Aliases: []string{"del", "rm"},
	Run: func(cmd *cobra.Command, args []string) {
		identifier := args[0]
		logger.Info("Executing 'platform delete' command for identifier: %s", identifier)

		var result sql.Result
		var err error
		var deletedBy string
		var query string
		var dbArgs []interface{}

		// Try parsing as ID first
		targetID, parseErr := strconv.ParseInt(identifier, 10, 64)
		if parseErr == nil {
			// It's a numeric ID
			deletedBy = fmt.Sprintf("ID %d", targetID)
			query = "DELETE FROM platforms WHERE id = ?"
			dbArgs = append(dbArgs, targetID)
		} else {
			// Not numeric, assume it's a name
			deletedBy = fmt.Sprintf("name '%s'", identifier)
			query = "DELETE FROM platforms WHERE LOWER(name) = LOWER(?)"
			dbArgs = append(dbArgs, identifier)
		}

		// Optional: Add a confirmation prompt here for safety
		// fmt.Printf("WARNING: This will delete platform '%s' and ALL associated targets and scope rules.\nAre you sure? (yes/no): ", identifier)
		// var confirmation string
		// fmt.Scanln(&confirmation)
		// if strings.ToLower(strings.TrimSpace(confirmation)) != "yes" {
		// 	fmt.Println("Deletion cancelled.")
		// 	os.Exit(0)
		// }

		logger.Info("Attempting to delete platform by %s", deletedBy)

		stmt, errPrep := database.DB.Prepare(query)
		if errPrep != nil {
			logger.Error("platform delete: Error preparing delete statement for platform %s: %v", deletedBy, errPrep)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		result, err = stmt.Exec(dbArgs...)
		if err != nil {
			logger.Error("platform delete: Error executing delete statement for platform %s: %v", deletedBy, err)
			fmt.Fprintln(os.Stderr, "Error deleting platform from database.")
			os.Exit(1)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			logger.Error("platform delete: Error getting rows affected for platform %s: %v", deletedBy, err)
			fmt.Printf("Successfully deleted platform %s (unable to verify rows affected).\n", deletedBy)
			logger.Info("Platform deleted via CLI: %s", deletedBy)
			return
		}

		if rowsAffected == 0 {
			fmt.Fprintf(os.Stderr, "Error: Platform %s not found.\n", deletedBy)
			os.Exit(1)
		}

		fmt.Printf("Successfully deleted platform %s.\n", deletedBy)
		logger.Info("Platform deleted via CLI: %s", deletedBy)
	},
}

// --- Init Function ---

func init() {
	// Add flags for add/update commands (shared variable)
	platformAddCmd.Flags().StringVarP(&platformName, "name", "n", "", "Name of the platform (required for add, required for update)")
	platformUpdateCmd.Flags().StringVarP(&platformName, "name", "n", "", "New name for the platform (required for update)")
	platformAddCmd.MarkFlagRequired("name")
	platformUpdateCmd.MarkFlagRequired("name")

	// Add subcommands to the base platform command
	platformCmd.AddCommand(platformListCmd)
	platformCmd.AddCommand(platformAddCmd)
	platformCmd.AddCommand(platformGetCmd)
	platformCmd.AddCommand(platformUpdateCmd)
	platformCmd.AddCommand(platformDeleteCmd)

	// Add the base platform command to the root command
	rootCmd.AddCommand(platformCmd)
}