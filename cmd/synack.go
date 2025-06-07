package cmd

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	// "strconv" // Not used
	"strings"
	"text/tabwriter"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/spf13/cobra"
)

// --- Flags ---
var (
	synackListStatus     string
	synackListLimit      int
	synackListPage       int
	synackListActiveOnly bool
)

// --- Base Command ---
var synackCmd = &cobra.Command{
	Use:     "synack",
	Short:   "Manage Synack-specific targets and data",
	Long:    `List and manage targets discovered from the Synack platform.`,
	Aliases: []string{"srt"},
}

// --- List Synack Targets Subcommand ---
var synackListTargetsCmd = &cobra.Command{
	Use:     "list",
	Short:   "List targets captured from Synack",
	Long:    `Retrieves and displays targets stored in the synack_targets table, with optional filters for status and activity.`,
	Aliases: []string{"ls", "targets"},
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'synack list targets' command")

		if synackListPage < 1 {
			synackListPage = 1
		}
		if synackListLimit < 1 {
			synackListLimit = 30 // Default limit
		}
		offset := (synackListPage - 1) * synackListLimit

		// --- Calculate Total Count and Pages ---
		countQuery := `SELECT COUNT(*) FROM synack_targets`
		countConditions := []string{}
		countArgs := []interface{}{}

		if synackListActiveOnly {
			countConditions = append(countConditions, "is_active = TRUE")
		}
		if synackListStatus != "" {
			countConditions = append(countConditions, "status = ?")
			countArgs = append(countArgs, synackListStatus)
		}

		if len(countConditions) > 0 {
			countQuery += " WHERE " + strings.Join(countConditions, " AND ")
		}

		var totalRecords int64
		err := database.DB.QueryRow(countQuery, countArgs...).Scan(&totalRecords)
		if err != nil {
			logger.Error("Failed to count total Synack target records: %v", err)
			fmt.Fprintln(os.Stderr, "Error counting Synack target records.")
			os.Exit(1)
		}

		totalPages := int64(0)
		if totalRecords > 0 && synackListLimit > 0 {
			totalPages = int64(math.Ceil(float64(totalRecords) / float64(synackListLimit)))
		} else if totalRecords > 0 && synackListLimit <= 0 { // Show all if limit is 0 or less
			totalPages = 1
		}
		if synackListPage > int(totalPages) && totalPages > 0 {
			synackListPage = int(totalPages)
			offset = (synackListPage - 1) * synackListLimit
		}
		// --- End Count Calculation ---

		// --- Build Query for fetching data ---
		query := `SELECT id, synack_target_id_str, codename, name, category, status, is_active, first_seen_timestamp, last_seen_timestamp FROM synack_targets`
		conditions := []string{} // For main query WHERE
		dbArgs := []interface{}{}   // For main query args

		if synackListActiveOnly {
			conditions = append(conditions, "is_active = TRUE")
		}
		if synackListStatus != "" {
			conditions = append(conditions, "status = ?")
			dbArgs = append(dbArgs, synackListStatus)
		}

		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
		query += " ORDER BY codename ASC" // MODIFIED: Default sort order

		if synackListLimit > 0 {
			query += " LIMIT ? OFFSET ?"
			dbArgs = append(dbArgs, synackListLimit, offset)
		}

		logger.Debug("Executing Synack list query: %s with args: %v", query, dbArgs)
		rows, err := database.DB.Query(query, dbArgs...)
		if err != nil {
			logger.Error("Failed to query Synack targets: %v", err)
			fmt.Fprintln(os.Stderr, "Error retrieving Synack targets from database.")
			os.Exit(1)
		}
		defer rows.Close()

		targets := []models.SynackTarget{}
		for rows.Next() {
			var t models.SynackTarget
			var firstSeenStr, lastSeenStr string
			var codename, name, category, status sql.NullString // Handle nullable text fields

			if err := rows.Scan(
				&t.DBID, &t.SynackTargetIDStr, &codename, &name, &category, &status,
				&t.IsActive, &firstSeenStr, &lastSeenStr,
			); err != nil {
				logger.Error("Failed to scan Synack target row: %v", err)
				fmt.Fprintln(os.Stderr, "Error reading Synack target data from database.")
				os.Exit(1)
			}
			if codename.Valid { t.Codename = codename.String }
			if name.Valid { t.Name = name.String }
			if category.Valid { t.Category = category.String }
			if status.Valid { t.Status = status.String }

			parsedTime, parseErr := time.Parse(time.RFC3339, firstSeenStr)
			if parseErr != nil { parsedTime, _ = time.Parse("2006-01-02 15:04:05", firstSeenStr) }
			t.FirstSeenTimestamp = parsedTime

			parsedTime, parseErr = time.Parse(time.RFC3339, lastSeenStr)
			if parseErr != nil { parsedTime, _ = time.Parse("2006-01-02 15:04:05", lastSeenStr) }
			t.LastSeenTimestamp = parsedTime
			
			targets = append(targets, t)
		}
		if err = rows.Err(); err != nil {
			logger.Error("Error iterating Synack target rows: %v", err)
			fmt.Fprintln(os.Stderr, "Error iterating Synack target results.")
			os.Exit(1)
		}

		if len(targets) == 0 {
			fmt.Println("No Synack targets found matching the criteria.")
			return
		}

		fmt.Println("Synack Targets:")
		writer := new(tabwriter.Writer)
		writer.Init(os.Stdout, 0, 8, 1, '\t', 0)
		fmt.Fprintln(writer, "DB_ID\tSYNACK_ID\tCODENAME\tNAME\tCATEGORY\tSTATUS\tACTIVE\tLAST_SEEN")
		fmt.Fprintln(writer, "-----\t---------\t--------\t----\t--------\t------\t------\t---------")
		for _, t := range targets {
			activeStr := "No"
			if t.IsActive { activeStr = "Yes" }
			lastSeenFmt := "N/A"
			if !t.LastSeenTimestamp.IsZero() { lastSeenFmt = t.LastSeenTimestamp.Format("2006-01-02 15:04") }

			displayName := t.Name
			if len(displayName) > 30 { displayName = displayName[:27]+"..." }
			displayCodename := t.Codename
			if len(displayCodename) > 25 { displayCodename = displayCodename[:22]+"..." }


			fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				t.DBID, t.SynackTargetIDStr, displayCodename, displayName, t.Category, t.Status, activeStr, lastSeenFmt,
			)
		}
		writer.Flush()

		// Print Footer
		fmt.Println("---")
		footer := fmt.Sprintf("Page %d / %d (%d total Synack targets matching DB filters)", synackListPage, totalPages, totalRecords)
		fmt.Println(footer)

		baseCmd := "toolkit synack list"
		if synackListActiveOnly { baseCmd += " --active" }
		if synackListStatus != "" { baseCmd += fmt.Sprintf(" --status %s", synackListStatus) }
		if synackListLimit != 30 { baseCmd += fmt.Sprintf(" --limit %d", synackListLimit) }


		if synackListPage > 1 {
			fmt.Printf("  Previous page: %s --page %d\n", baseCmd, synackListPage-1)
		}
		if int64(synackListPage) < totalPages {
			fmt.Printf("  Next page:     %s --page %d\n", baseCmd, synackListPage+1)
		}
		fmt.Println("---")

		logger.Info("Successfully listed %d Synack targets (Page %d)", len(targets), synackListPage)
	},
}

func init() {
	synackListTargetsCmd.Flags().StringVarP(&synackListStatus, "status", "s", "", "Filter Synack targets by status (e.g., new, assessing)")
	synackListTargetsCmd.Flags().BoolVar(&synackListActiveOnly, "active", false, "Only show currently active Synack targets")
	synackListTargetsCmd.Flags().IntVarP(&synackListLimit, "limit", "l", 30, "Number of results per page")
	synackListTargetsCmd.Flags().IntVarP(&synackListPage, "page", "p", 1, "Page number to retrieve")

	synackCmd.AddCommand(synackListTargetsCmd)
	rootCmd.AddCommand(synackCmd)
}