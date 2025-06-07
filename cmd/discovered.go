package cmd

import (
	// "database/sql" // Not needed here
	// "errors"       // REMOVED
	"fmt"
	"math" // Needed for Ceil
	"os"
	// "regexp" // REMOVED
	"strconv" // Needed for page/limit/targetID parsing
	// "strings" // Not needed here
	"text/tabwriter"
	"time"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/spf13/cobra"
)

// --- Flags ---
var (
	// Flags for list urls
	discoveredListTargetID int64
	discoveredListLimit    int
	discoveredListPage     int
)

// --- Base Command ---

// discoveredCmd represents the base command for discovered asset operations
var discoveredCmd = &cobra.Command{
	Use:     "discovered",
	Short:   "Manage discovered assets (URLs, etc.)",
	Long:    `List and manage assets discovered through analysis or other means.`,
	Aliases: []string{"disc"},
}

// --- List URLs Subcommand ---

var discoveredListUrlsCmd = &cobra.Command{
	Use:     "list urls", // Use two words for the command
	Short:   "List discovered URLs for a target",
	Long: `Retrieves and displays URLs discovered via analysis tools (like jsluice) 
for a specific target. Requires --target-id or a currently set target.`,
	Aliases: []string{"ls urls", "urls"}, // Add aliases
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'discovered list urls' command")

		targetIDToList := discoveredListTargetID // Start with flag value

		// If flag was not set, try to get from state file
		if !cmd.Flags().Changed("target-id") {
			logger.Info("discovered list urls: --target-id flag not provided, attempting to use current target from state.")
			state, err := readState() // Assumes readState helper is available (defined in cmd/target.go)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading state file: %v\n", err)
				fmt.Fprintln(os.Stderr, "Please set a current target using 'toolkit target set-current' or provide --target-id.")
				os.Exit(1)
			}
			if state.CurrentTargetID == 0 {
				fmt.Fprintln(os.Stderr, "Error: No current target set and --target-id flag not provided.")
				fmt.Fprintln(os.Stderr, "Please set a current target using 'toolkit target set-current' or provide --target-id.")
				os.Exit(1)
			}
			targetIDToList = state.CurrentTargetID
			logger.Info("discovered list urls: Using current target ID from state: %d", targetIDToList)
		} else {
			logger.Info("discovered list urls: Using target ID from flag: %d", targetIDToList)
		}

		if targetIDToList == 0 {
			fmt.Fprintln(os.Stderr, "Error: Target ID cannot be zero.")
			os.Exit(1)
		}

		// Validate pagination flags
		if discoveredListPage < 1 {
			discoveredListPage = 1
		}
		if discoveredListLimit < 1 {
			discoveredListLimit = 50 // Default limit for this command
		}
		offset := (discoveredListPage - 1) * discoveredListLimit

		// Count total records for pagination
		var totalRecords int64
		countQuery := "SELECT COUNT(*) FROM discovered_urls WHERE target_id = ?"
		err := database.DB.QueryRow(countQuery, targetIDToList).Scan(&totalRecords)
		if err != nil {
			logger.Error("Failed to count discovered URLs for target %d: %v", targetIDToList, err)
			fmt.Fprintf(os.Stderr, "Error counting discovered URLs for target %d.\n", targetIDToList)
			os.Exit(1)
		}

		totalPages := int64(0)
		if totalRecords > 0 {
			totalPages = int64(math.Ceil(float64(totalRecords) / float64(discoveredListLimit))) // Uses math.Ceil
		}
		if discoveredListPage > int(totalPages) && totalPages > 0 {
			discoveredListPage = int(totalPages)
			offset = (discoveredListPage - 1) * discoveredListLimit // Recalculate offset
		}

		// Query the database for discovered URLs for the specified target
		query := `SELECT id, http_traffic_log_id, url, source_type, discovered_at 
                  FROM discovered_urls 
                  WHERE target_id = ? 
                  ORDER BY target_id ASC, url ASC -- Changed sort order
                  LIMIT ? OFFSET ?`

		logger.Debug("Executing query: %s with args: [%d, %d, %d]", query, targetIDToList, discoveredListLimit, offset)
		rows, err := database.DB.Query(query, targetIDToList, discoveredListLimit, offset)
		if err != nil {
			logger.Error("Failed to query discovered URLs for target %d: %v", targetIDToList, err)
			fmt.Fprintf(os.Stderr, "Error retrieving discovered URLs for target %d.\n", targetIDToList)
			os.Exit(1)
		}
		defer rows.Close()

		discoveredURLs := []models.DiscoveredURL{}
		for rows.Next() {
			var du models.DiscoveredURL
			var timestampStr string
			// target_id is known, set it directly
			du.TargetID = &targetIDToList

			if err := rows.Scan(&du.ID, &du.HTTPTrafficLogID, &du.URL, &du.SourceType, &timestampStr); err != nil {
				logger.Error("Failed to scan discovered URL row: %v", err)
				fmt.Fprintln(os.Stderr, "Error reading discovered URL data.")
				os.Exit(1) // Exit on scan error
			}
			// Parse timestamp
			parsedTime, parseErr := time.Parse(time.RFC3339, timestampStr)
			if parseErr != nil {
				parsedTime, parseErr = time.Parse("2006-01-02 15:04:05", timestampStr)
			}
			if parseErr == nil {
				du.DiscoveredAt = parsedTime
			} else {
				logger.Error("Failed to parse discovered_at timestamp '%s': %v", timestampStr, parseErr)
			}

			discoveredURLs = append(discoveredURLs, du)
		}
		if err = rows.Err(); err != nil {
			logger.Error("Error iterating discovered URL rows for target %d: %v", targetIDToList, err)
			fmt.Fprintf(os.Stderr, "Error iterating discovered URL results for target %d.\n", targetIDToList)
			os.Exit(1)
		}

		if len(discoveredURLs) == 0 {
			fmt.Printf("No discovered URLs found for target ID %d.\n", targetIDToList)
			return
		}

		// Print results
		fmt.Printf("Discovered URLs for Target ID %d:\n", targetIDToList)
		writer := new(tabwriter.Writer)
		writer.Init(os.Stdout, 0, 8, 1, '\t', 0)

		fmt.Fprintln(writer, "ID\tTARGET_ID\tLOG_ID\tSOURCE\tDISCOVERED_AT\tURL")
		fmt.Fprintln(writer, "--\t---------\t------\t------\t-------------\t---")

		for _, du := range discoveredURLs {
			tsFormatted := "N/A"
			if !du.DiscoveredAt.IsZero() {
				tsFormatted = du.DiscoveredAt.Format("2006-01-02 15:04:05")
			}
			// Safely dereference TargetID pointer for printing
			targetIDStr := "N/A"
			if du.TargetID != nil {
				targetIDStr = strconv.FormatInt(*du.TargetID, 10)
			}

			fmt.Fprintf(writer, "%d\t%s\t%d\t%s\t%s\t%s\n",
				du.ID,
				targetIDStr, // Print the Target ID
				du.HTTPTrafficLogID,
				du.SourceType,
				tsFormatted,
				du.URL,
			)
		}
		writer.Flush()

		// Print Footer
		fmt.Println("---")
		footer := fmt.Sprintf("Page %d / %d (%d total URLs found for this target)", discoveredListPage, totalPages, totalRecords)
		fmt.Println(footer)

		baseCmd := fmt.Sprintf("toolkit discovered list urls --target-id %d", targetIDToList)
		if discoveredListLimit != 50 { // Default limit for this command
			baseCmd += fmt.Sprintf(" --limit %d", discoveredListLimit)
		}

		if discoveredListPage > 1 {
			fmt.Printf("  Previous page: %s --page %d\n", baseCmd, discoveredListPage-1)
		}
		if int64(discoveredListPage) < totalPages {
			fmt.Printf("  Next page:     %s --page %d\n", baseCmd, discoveredListPage+1)
		}
		fmt.Println("---")

		logger.Info("Successfully listed %d discovered URLs for target %d (Page %d)", len(discoveredURLs), targetIDToList, discoveredListPage)
	},
}

// --- Init Function ---

func init() {
	// Add flags to the list urls command
	discoveredListUrlsCmd.Flags().Int64VarP(&discoveredListTargetID, "target-id", "t", 0, "ID of the target to list discovered URLs for (uses current target if not specified)")
	discoveredListUrlsCmd.Flags().IntVarP(&discoveredListLimit, "limit", "l", 50, "Number of results per page")
	discoveredListUrlsCmd.Flags().IntVarP(&discoveredListPage, "page", "p", 1, "Page number to retrieve")

	// Add subcommands to the base discovered command
	discoveredCmd.AddCommand(discoveredListUrlsCmd)
	// Add other discovered subcommands here later (e.g., list secrets, list paths)

	// Add the base discovered command to the root command
	rootCmd.AddCommand(discoveredCmd)
}