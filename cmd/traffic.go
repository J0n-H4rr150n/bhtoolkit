package cmd

import (
	"bufio" // For trafficPurgeCmd confirmation
	"bytes" // For printBody helper
	"database/sql"
	"encoding/json" // For printHeaders and printBody helpers
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"toolkit/core"
	"toolkit/database"
	"toolkit/logger"
	"toolkit/models"

	"github.com/spf13/cobra"
)

// --- Flags ---
var (
	// List flags
	trafficListDomain     string
	trafficListRegex      string
	trafficListRegexField string
	trafficListStatusCode int
	trafficListLimit      int
	trafficListPage       int
	// Map flags / List Mapped flags
	trafficMapTargetID int64
	// Analyze flags
	trafficAnalyzeTool string
	// Purge flags
	trafficPurgeForce bool
)

// --- Base Command ---

var trafficCmd = &cobra.Command{
	Use:     "traffic",
	Short:   "View and manage captured HTTP traffic",
	Long:    `Allows viewing, filtering, mapping, analyzing, getting details, purging, and potentially managing the HTTP requests and responses captured by the proxy.`,
	Aliases: []string{"tf"},
}

// --- Helper to print headers nicely (package level) ---
func printHeaders(headersJSON string) {
	if headersJSON == "" {
		// fmt.Println("(No Headers)") // Keep output clean if empty
		return
	}
	var headers map[string][]string
	err := json.Unmarshal([]byte(headersJSON), &headers) // Uses json
	if err != nil {
		logger.Error("Error unmarshalling headers JSON: %v. Raw: %s", err, headersJSON)
		fmt.Println(headersJSON) // Print raw if unmarshal fails
		return
	}
	for name, values := range headers {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}
}

// --- Helper to print body (with basic JSON pretty-printing) (package level) ---
func printBody(bodyBytes []byte, contentType string) {
	if len(bodyBytes) == 0 {
		// fmt.Println("(No Body)") // Print nothing if body is empty
		return
	}

	if strings.Contains(strings.ToLower(contentType), "json") {
		var prettyJSON bytes.Buffer                                           // Uses bytes
		if err := json.Indent(&prettyJSON, bodyBytes, "", "  "); err == nil { // Uses json
			fmt.Println(prettyJSON.String())
			return
		}
		logger.Debug("Failed to pretty-print JSON body, printing as string.")
	}

	// Print as string, replacing invalid UTF-8 sequences
	printableBody := strings.ToValidUTF8(string(bodyBytes), "") // Use replacement char
	fmt.Println(printableBody)
}

// --- List Command ---
var trafficListCmd = &cobra.Command{
	Use:   "list",
	Short: "List captured HTTP traffic logs with filters and pagination",
	Long: `Retrieves and displays entries from the HTTP traffic log, with optional filters.
Supports pagination using --page and --limit flags.
Regex filter can be applied to different fields using --regex-field.
Note: Total page count is based on database filters (--domain, --status-code) only, not the --regex filter which is applied after fetching.`,
	Aliases: []string{"ls"},
	Example: `  # List the 30 most recent traffic entries
  toolkit traffic list

  # List the second page of results
  toolkit traffic list --page 2

  # List page 3 with 20 items per page
  toolkit traffic list --limit 20 --page 3

  # List traffic for a domain
  toolkit traffic list --domain example.com

  # List 404 errors for a domain
  toolkit traffic list -d example.com --status-code 404

  # List traffic where the URL path starts with /api/v1/
  toolkit traffic list --regex "/api/v1/" --regex-field url

  # List traffic where the response body contains 'error_code'
  toolkit traffic list --regex "error_code" --regex-field res_body

  # List traffic where request headers contain 'X-Api-Key' (case-insensitive regex)
  toolkit traffic list -r "(?i)x-api-key" -f req_headers`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'traffic list' command")

		// Validate page and limit
		if trafficListPage < 1 {
			trafficListPage = 1
		}
		if trafficListLimit < 1 {
			logger.Info("Invalid limit %d provided, using default 30", trafficListLimit)
			trafficListLimit = 30
		}
		// Validate regex field
		validRegexFields := map[string]bool{
			"url":         true,
			"req_headers": true,
			"req_body":    true,
			"res_headers": true,
			"res_body":    true,
		}
		trafficListRegexField = strings.ToLower(strings.TrimSpace(trafficListRegexField))
		if trafficListRegex != "" && !validRegexFields[trafficListRegexField] {
			fmt.Fprintf(os.Stderr, "Error: Invalid --regex-field '%s'. Must be one of: url, req_headers, req_body, res_headers, res_body\n", trafficListRegexField)
			os.Exit(1)
		}

		// --- Calculate Total Count and Pages ---
		countQuery := `SELECT COUNT(*) FROM http_traffic_log`
		countConditions := []string{}
		countArgs := []interface{}{}

		if trafficListDomain != "" {
			countConditions = append(countConditions, "request_url LIKE ?")
			countArgs = append(countArgs, "%"+trafficListDomain+"%")
		}
		if trafficListStatusCode > 0 {
			countConditions = append(countConditions, "response_status_code = ?")
			countArgs = append(countArgs, trafficListStatusCode)
		}

		if len(countConditions) > 0 {
			countQuery += " WHERE " + strings.Join(countConditions, " AND ")
		}

		var totalRecords int64
		err := database.DB.QueryRow(countQuery, countArgs...).Scan(&totalRecords)
		if err != nil {
			logger.Error("Failed to count total traffic log records: %v", err)
			fmt.Fprintln(os.Stderr, "Error counting traffic log records.")
			os.Exit(1)
		}

		totalPages := int64(0)
		if totalRecords > 0 && trafficListLimit > 0 {
			totalPages = int64(math.Ceil(float64(totalRecords) / float64(trafficListLimit)))
		} else if totalRecords > 0 && trafficListLimit <= 0 {
			totalPages = 1
		}

		if trafficListPage > int(totalPages) && totalPages > 0 {
			logger.Info("Requested page %d exceeds total pages %d. Showing last page.", trafficListPage, totalPages)
			trafficListPage = int(totalPages)
		}
		// --- End Count Calculation ---

		// --- Build Query for fetching data ---
		offset := (trafficListPage - 1) * trafficListLimit
		// Select all fields needed for display AND potential regex filtering
		query := `SELECT id, timestamp, request_method, request_url, response_status_code, 
                         response_content_type, response_body_size, duration_ms, is_https,
                         request_headers, request_body, response_headers, response_body 
                  FROM http_traffic_log`
		conditions := []string{}
		dbArgs := []interface{}{}

		if trafficListDomain != "" {
			conditions = append(conditions, "request_url LIKE ?")
			dbArgs = append(dbArgs, "%"+trafficListDomain+"%")
		}
		if trafficListStatusCode > 0 {
			conditions = append(conditions, "response_status_code = ?")
			dbArgs = append(dbArgs, trafficListStatusCode)
		}

		var regexFilter *regexp.Regexp
		var regexErr error
		if trafficListRegex != "" {
			logger.Info("Applying regex filter '%s' post-fetch on field '%s'", trafficListRegex, trafficListRegexField)
			regexFilter, regexErr = regexp.Compile(trafficListRegex)
			if regexErr != nil {
				logger.Error("Invalid regex pattern provided: %v", regexErr)
				fmt.Fprintf(os.Stderr, "Error: Invalid regex pattern: %v\n", regexErr)
				os.Exit(1)
			}
		}

		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}
		query += " ORDER BY timestamp DESC"

		if trafficListLimit > 0 {
			query += " LIMIT ? OFFSET ?"
			dbArgs = append(dbArgs, trafficListLimit, offset)
		} else {
			logger.Info("Limit is %d, fetching all matching records.", trafficListLimit)
		}

		logger.Debug("Executing query: %s with args: %v", query, dbArgs)
		rows, err := database.DB.Query(query, dbArgs...)
		if err != nil {
			logger.Error("Failed to query traffic log: %v", err)
			fmt.Fprintln(os.Stderr, "Error retrieving traffic logs from database.")
			os.Exit(1)
		}
		defer rows.Close()

		logs := []models.HTTPTrafficLog{}
		fetchedCount := 0
		for rows.Next() {
			fetchedCount++
			var t models.HTTPTrafficLog
			var statusCode sql.NullInt64
			var contentType sql.NullString
			var bodySize sql.NullInt64
			var duration sql.NullInt64
			var isHTTPS sql.NullBool
			var timestampStr string
			var reqHeadersStr, resHeadersStr sql.NullString
			var reqBodyBytes, resBodyBytes []byte

			err := rows.Scan(
				&t.ID, &timestampStr, &t.RequestMethod, &t.RequestURL, &statusCode,
				&contentType, &bodySize, &duration, &isHTTPS,
				&reqHeadersStr, &reqBodyBytes, &resHeadersStr, &resBodyBytes,
			)
			if err != nil {
				logger.Error("Failed to scan traffic log row: %v", err)
				fmt.Fprintln(os.Stderr, "Error reading traffic log data from database.")
				os.Exit(1)
			}

			parsedTime, parseErr := time.Parse(time.RFC3339, timestampStr)
			if parseErr != nil {
				parsedTime, parseErr = time.Parse("2006-01-02 15:04:05", timestampStr)
			}
			if parseErr != nil {
				logger.Error("Failed to parse timestamp string '%s': %v", timestampStr, parseErr)
			} else {
				t.Timestamp = parsedTime
			}
			if statusCode.Valid {
				t.ResponseStatusCode = int(statusCode.Int64)
			} // t.RequestMethod and t.RequestURL are now sql.NullString
			if contentType.Valid {
				t.ResponseContentType = contentType.String
			}
			if bodySize.Valid {
				t.ResponseBodySize = bodySize.Int64
			}
			if duration.Valid {
				t.DurationMs = duration.Int64
			}
			if isHTTPS.Valid {
				t.IsHTTPS = isHTTPS.Bool
			}
			if reqHeadersStr.Valid {
				t.RequestHeaders = reqHeadersStr
			}
			if resHeadersStr.Valid {
				t.ResponseHeaders = resHeadersStr.String
			}
			t.RequestBody = reqBodyBytes
			t.ResponseBody = resBodyBytes

			if regexFilter != nil {
				match := false
				switch trafficListRegexField {
				case "url": // t.RequestURL is now sql.NullString
					match = t.RequestURL.Valid && regexFilter.MatchString(t.RequestURL.String)
				case "req_headers":
					match = t.RequestHeaders.Valid && regexFilter.MatchString(t.RequestHeaders.String)
				case "req_body":
					match = regexFilter.Match(t.RequestBody)
				case "res_headers":
					match = regexFilter.MatchString(t.ResponseHeaders)
				case "res_body":
					match = regexFilter.Match(t.ResponseBody)
				default: // Default to URL if field is invalid or not specified with regex
					match = t.RequestURL.Valid && regexFilter.MatchString(t.RequestURL.String)
				}

				if match {
					logs = append(logs, t)
				}
			} else {
				logs = append(logs, t)
			}
		}
		if err = rows.Err(); err != nil {
			logger.Error("Error iterating traffic log rows: %v", err)
			fmt.Fprintln(os.Stderr, "Error iterating traffic log results.")
			os.Exit(1)
		}

		if len(logs) == 0 && fetchedCount == 0 {
			fmt.Println("No matching traffic logs found for the specified criteria.")
			return
		}
		if len(logs) == 0 && fetchedCount > 0 && regexFilter != nil {
			fmt.Println("No traffic logs on this page matched the regex filter.")
		}

		if len(logs) > 0 {
			fmt.Println("Captured HTTP Traffic:")
			writer := new(tabwriter.Writer)
			writer.Init(os.Stdout, 0, 8, 1, '\t', 0)

			fmt.Fprintln(writer, "ID\tTIMESTAMP\tMETH\tURL\tSTATUS\tCONTENT_TYPE\tSIZE\tDURATION\tHTTPS")
			fmt.Fprintln(writer, "--\t---------\t----\t---\t------\t------------\t----\t--------\t-----")

			for _, t := range logs {
				httpsStr := "N"
				if t.IsHTTPS {
					httpsStr = "Y"
				}
				tsFormatted := "N/A"
				if !t.Timestamp.IsZero() {
					tsFormatted = t.Timestamp.Format("2006-01-02 15:04:05")
				}
				displayURL := t.RequestURL.String // Use .String
				if len(displayURL) > 80 {
					displayURL = displayURL[:77] + "..."
				}
				displayContentType := t.ResponseContentType
				if len(displayContentType) > 30 {
					displayContentType = displayContentType[:27] + "..."
				}
				fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%d\t%s\t%d\t%dms\t%s\n",
					t.ID, tsFormatted, t.RequestMethod.String, displayURL, t.ResponseStatusCode, // Use .String
					displayContentType, t.ResponseBodySize, t.DurationMs, httpsStr,
				)
			}
			writer.Flush()
		}

		fmt.Println("---")
		footer := fmt.Sprintf("Page %d / %d (%d total records matching DB filters", trafficListPage, totalPages, totalRecords)
		if regexFilter != nil {
			footer += fmt.Sprintf(", %d shown after regex on field '%s')", len(logs), trafficListRegexField)
		} else {
			footer += ")"
		}
		fmt.Println(footer)

		baseCmd := "toolkit traffic list"
		if trafficListDomain != "" {
			baseCmd += fmt.Sprintf(" --domain \"%s\"", trafficListDomain)
		}
		if trafficListStatusCode > 0 {
			baseCmd += fmt.Sprintf(" --status-code %d", trafficListStatusCode)
		}
		if trafficListRegex != "" {
			baseCmd += fmt.Sprintf(" --regex '%s' --regex-field %s", trafficListRegex, trafficListRegexField)
		}
		if trafficListLimit != 30 {
			baseCmd += fmt.Sprintf(" --limit %d", trafficListLimit)
		}

		if trafficListPage > 1 {
			fmt.Printf("  Previous page: %s --page %d\n", baseCmd, trafficListPage-1)
		}
		if int64(trafficListPage) < totalPages {
			fmt.Printf("  Next page:     %s --page %d\n", baseCmd, trafficListPage+1)
		}
		fmt.Println("---")

		logger.Info("Successfully listed %d traffic log entries (Page %d)", len(logs), trafficListPage)
	},
}

// --- Map Command ---
var trafficMapCmd = &cobra.Command{
	Use:   "map [log_id]",
	Short: "Map a traffic log entry to a target",
	Long: `Associates a specific HTTP traffic log entry (identified by its ID) with a target.
If --target-id is not provided, it uses the currently set target (see 'toolkit target current' and 'toolkit target set-current').
Will return an error if the log entry is already mapped to a target.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logIDStr := args[0]
		logger.Info("Executing 'traffic map' command for log ID: %s", logIDStr)

		targetIDToMap := trafficMapTargetID

		if !cmd.Flags().Changed("target-id") {
			logger.Debug("traffic map: --target-id flag not provided, attempting to use current target from state.")
			state, err := readState()
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
			targetIDToMap = state.CurrentTargetID
			logger.Debug("traffic map: Using current target ID from state: %d", targetIDToMap)
		} else {
			logger.Debug("traffic map: Using target ID from flag: %d", targetIDToMap)
		}

		if targetIDToMap == 0 {
			fmt.Fprintln(os.Stderr, "Error: Target ID to map to cannot be zero.")
			os.Exit(1)
		}

		logID, err := strconv.ParseInt(logIDStr, 10, 64)
		if err != nil {
			logger.Error("Invalid log_id provided: %s. Must be an integer.", logIDStr)
			fmt.Fprintf(os.Stderr, "Error: Invalid log_id '%s'. Must be an integer.\n", logIDStr)
			os.Exit(1)
		}

		var targetExists bool
		err = database.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM targets WHERE id = ?)", targetIDToMap).Scan(&targetExists)
		if err != nil {
			logger.Error("traffic map: Error checking target existence for TargetID %d: %v", targetIDToMap, err)
			fmt.Fprintln(os.Stderr, "Error verifying target ID.")
			os.Exit(1)
		}
		if !targetExists {
			fmt.Fprintf(os.Stderr, "Error: Target with ID %d does not exist.\n", targetIDToMap)
			os.Exit(1)
		}

		var currentTargetID sql.NullInt64
		err = database.DB.QueryRow("SELECT target_id FROM http_traffic_log WHERE id = ?", logID).Scan(&currentTargetID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fmt.Fprintf(os.Stderr, "Error: Traffic log entry with ID %d does not exist.\n", logID)
			} else {
				logger.Error("traffic map: Error checking log existence for LogID %d: %v", logID, err)
				fmt.Fprintln(os.Stderr, "Error verifying log entry ID.")
			}
			os.Exit(1)
		}

		if currentTargetID.Valid {
			if currentTargetID.Int64 == targetIDToMap {
				fmt.Printf("Info: Traffic log entry %d is already mapped to target %d.\n", logID, targetIDToMap)
				logger.Info("Traffic map: Log entry %d already mapped to target %d.", logID, targetIDToMap)
			} else {
				fmt.Fprintf(os.Stderr, "Error: Traffic log entry %d is already mapped to a different target (ID: %d).\n", logID, currentTargetID.Int64)
				logger.Error("Traffic map: Log entry %d already mapped to different target %d, cannot map to %d.", logID, currentTargetID.Int64, targetIDToMap)
			}
			os.Exit(1)
		}

		stmt, err := database.DB.Prepare("UPDATE http_traffic_log SET target_id = ? WHERE id = ?")
		if err != nil {
			logger.Error("traffic map: Error preparing update statement for log ID %d: %v", logID, err)
			fmt.Fprintln(os.Stderr, "Error preparing database operation.")
			os.Exit(1)
		}
		defer stmt.Close()

		result, err := stmt.Exec(targetIDToMap, logID)
		if err != nil {
			logger.Error("traffic map: Error executing update for log ID %d to target ID %d: %v", logID, targetIDToMap, err)
			fmt.Fprintln(os.Stderr, "Error updating traffic log entry.")
			os.Exit(1)
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected == 0 {
			fmt.Fprintf(os.Stderr, "Warning: Log entry %d was not found during update, though it existed before.\n", logID)
		} else {
			fmt.Printf("Successfully mapped traffic log entry %d to target %d.\n", logID, targetIDToMap)
			logger.Info("Traffic log entry %d mapped to target %d", logID, targetIDToMap)
		}
	},
}

// --- List Mapped Command ---
var trafficListMappedCmd = &cobra.Command{
	Use:   "list-mapped",
	Short: "List traffic log entries mapped to a specific target",
	Long: `Retrieves and displays HTTP traffic log entries that have been explicitly mapped
to the specified target ID (using --target-id) or the currently set target.`,
	Aliases: []string{"lsm"},
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'traffic list-mapped' command")

		targetIDToList := trafficMapTargetID

		if !cmd.Flags().Changed("target-id") {
			logger.Debug("traffic list-mapped: --target-id flag not provided, attempting to use current target from state.")
			state, err := readState()
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
			logger.Debug("traffic list-mapped: Using current target ID from state: %d", targetIDToList)
		} else {
			logger.Debug("traffic list-mapped: Using target ID from flag: %d", targetIDToList)
		}

		if targetIDToList == 0 {
			fmt.Fprintln(os.Stderr, "Error: Target ID to list mappings for cannot be zero.")
			os.Exit(1)
		}

		query := `SELECT id, timestamp, request_method, request_url, response_status_code, 
                         response_content_type, response_body_size, duration_ms, is_https 
                  FROM http_traffic_log
                  WHERE target_id = ? 
                  ORDER BY timestamp DESC`

		logger.Debug("Executing query: %s with args: [%d]", query, targetIDToList)
		rows, err := database.DB.Query(query, targetIDToList)
		if err != nil {
			logger.Error("Failed to query mapped traffic log for target %d: %v", targetIDToList, err)
			fmt.Fprintf(os.Stderr, "Error retrieving mapped traffic logs for target %d.\n", targetIDToList)
			os.Exit(1)
		}
		defer rows.Close()

		logs := []models.HTTPTrafficLog{}
		for rows.Next() {
			var t models.HTTPTrafficLog
			var statusCode sql.NullInt64
			var contentType sql.NullString
			var bodySize sql.NullInt64
			var duration sql.NullInt64
			var isHTTPS sql.NullBool // t.RequestMethod and t.RequestURL are now sql.NullString
			var timestampStr string

			if err := rows.Scan(&t.ID, &timestampStr, &t.RequestMethod, &t.RequestURL, &statusCode, &contentType, &bodySize, &duration, &isHTTPS); err != nil {
				logger.Error("Failed to scan mapped traffic log row: %v", err)
				fmt.Fprintln(os.Stderr, "Error reading mapped traffic log data.")
				os.Exit(1)
			}
			parsedTime, parseErr := time.Parse(time.RFC3339, timestampStr)
			if parseErr != nil {
				parsedTime, parseErr = time.Parse("2006-01-02 15:04:05", timestampStr)
			}
			if parseErr == nil {
				t.Timestamp = parsedTime
			} else {
				logger.Error("Failed to parse timestamp '%s': %v", timestampStr, parseErr)
			}
			if statusCode.Valid {
				t.ResponseStatusCode = int(statusCode.Int64)
			}
			if contentType.Valid {
				t.ResponseContentType = contentType.String
			}
			if bodySize.Valid {
				t.ResponseBodySize = bodySize.Int64
			}
			if duration.Valid {
				t.DurationMs = duration.Int64
			}
			if isHTTPS.Valid {
				t.IsHTTPS = isHTTPS.Bool
			}
			t.TargetID = &targetIDToList
			logs = append(logs, t)
		}
		if err = rows.Err(); err != nil {
			logger.Error("Error iterating mapped traffic log rows for target %d: %v", targetIDToList, err)
			fmt.Fprintf(os.Stderr, "Error iterating mapped traffic results for target %d.\n", targetIDToList)
			os.Exit(1)
		}

		if len(logs) == 0 {
			fmt.Printf("No traffic log entries found mapped to target ID %d.\n", targetIDToList)
			return
		}

		fmt.Printf("Traffic Log Entries Mapped to Target ID %d:\n", targetIDToList)
		writer := new(tabwriter.Writer)
		writer.Init(os.Stdout, 0, 8, 1, '\t', 0)

		fmt.Fprintln(writer, "ID\tTIMESTAMP\tMETH\tURL\tSTATUS\tCONTENT_TYPE\tSIZE\tDURATION\tHTTPS")
		fmt.Fprintln(writer, "--\t---------\t----\t---\t------\t------------\t----\t--------\t-----")

		for _, t := range logs {
			httpsStr := "N"
			if t.IsHTTPS {
				httpsStr = "Y"
			}
			tsFormatted := "N/A"
			if !t.Timestamp.IsZero() {
				tsFormatted = t.Timestamp.Format("2006-01-02 15:04:05")
			}
			displayURL := t.RequestURL.String // Use .String
			if len(displayURL) > 80 {
				displayURL = displayURL[:77] + "..."
			}
			displayContentType := t.ResponseContentType
			if len(displayContentType) > 30 {
				displayContentType = displayContentType[:27] + "..."
			}

			fmt.Fprintf(writer, "%d\t%s\t%s\t%s\t%d\t%s\t%d\t%dms\t%s\n",
				t.ID, tsFormatted, t.RequestMethod.String, displayURL, t.ResponseStatusCode, // Use .String
				displayContentType, t.ResponseBodySize, t.DurationMs, httpsStr,
			)
		}
		writer.Flush()
		logger.Info("Successfully listed %d mapped traffic log entries for target %d", len(logs), targetIDToList)
	},
}

// --- Get Command ---
var trafficGetCmd = &cobra.Command{
	Use:   "get [log_id]",
	Short: "Get full details for a specific traffic log entry",
	Long: `Retrieves and displays the full request and response (including headers and bodies) 
for a single traffic log entry identified by its ID.
For long bodies, pipe the output to a pager like 'less' (e.g., toolkit traffic get 123 | less).`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logIDStr := args[0]
		logger.Info("Executing 'traffic get' command for log ID: %s", logIDStr)

		logID, err := strconv.ParseInt(logIDStr, 10, 64)
		if err != nil {
			logger.Error("Invalid log_id provided: %s. Must be an integer.", logIDStr)
			fmt.Fprintf(os.Stderr, "Error: Invalid log_id '%s'. Must be an integer.\n", logIDStr)
			os.Exit(1)
		}

		var t models.HTTPTrafficLog
		var statusCode sql.NullInt64
		var contentType sql.NullString
		var bodySize sql.NullInt64
		var duration sql.NullInt64
		var isHTTPS sql.NullBool
		var timestampStr string
		var reqHeadersStr, resHeadersStr sql.NullString
		var reqBodyBytes, resBodyBytes []byte
		var targetID sql.NullInt64
		var notes sql.NullString
		var clientIPSql, serverIPSqL sql.NullString
		var reasonPhraseSql sql.NullString
		var reqHttpVerSql, resHttpVerSql sql.NullString

		query := `SELECT id, target_id, timestamp, request_method, request_url, request_http_version, 
                         request_headers, request_body, response_status_code, response_reason_phrase, 
                         response_http_version, response_headers, response_body, response_content_type, 
                         response_body_size, duration_ms, client_ip, server_ip, is_https, 
                         is_page_candidate, notes 
                  FROM http_traffic_log WHERE id = ?`

		err = database.DB.QueryRow(query, logID).Scan(
			&t.ID, &targetID, &timestampStr, &t.RequestMethod, &t.RequestURL, &reqHttpVerSql,
			&reqHeadersStr, &reqBodyBytes, &statusCode, &reasonPhraseSql,
			&resHttpVerSql, &resHeadersStr, &resBodyBytes, &contentType,
			&bodySize, &duration, &clientIPSql, &serverIPSqL, &isHTTPS,
			&t.IsPageCandidate, &notes,
		)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fmt.Fprintf(os.Stderr, "Error: Traffic log entry with ID %d not found.\n", logID)
			} else {
				logger.Error("traffic get: Error querying log ID %d: %v", logID, err)
				fmt.Fprintf(os.Stderr, "Error retrieving log entry %d from database.\n", logID)
			}
			os.Exit(1)
		}

		if targetID.Valid {
			t.TargetID = &targetID.Int64
		}
		parsedTime, parseErr := time.Parse(time.RFC3339, timestampStr)
		if parseErr != nil {
			parsedTime, parseErr = time.Parse("2006-01-02 15:04:05", timestampStr)
		}
		if parseErr == nil {
			t.Timestamp = parsedTime
		} else {
			logger.Error("Failed to parse timestamp '%s': %v", timestampStr, parseErr)
		}
		if reqHttpVerSql.Valid {
			t.RequestHTTPVersion = reqHttpVerSql.String
		}
		if reqHeadersStr.Valid {
			t.RequestHeaders = reqHeadersStr
		}
		t.RequestBody = reqBodyBytes
		if statusCode.Valid {
			t.ResponseStatusCode = int(statusCode.Int64)
		}
		if reasonPhraseSql.Valid {
			t.ResponseReasonPhrase = reasonPhraseSql.String
		}
		if resHttpVerSql.Valid {
			t.ResponseHTTPVersion = resHttpVerSql.String
		}
		if resHeadersStr.Valid {
			t.ResponseHeaders = resHeadersStr.String
		}
		t.ResponseBody = resBodyBytes
		if contentType.Valid {
			t.ResponseContentType = contentType.String
		}
		if bodySize.Valid {
			t.ResponseBodySize = bodySize.Int64
		}
		if duration.Valid {
			t.DurationMs = duration.Int64
		}
		if clientIPSql.Valid {
			t.ClientIP = clientIPSql.String
		}
		if serverIPSqL.Valid {
			t.ServerIP = serverIPSqL.String
		}
		if isHTTPS.Valid {
			t.IsHTTPS = isHTTPS.Bool
		}
		if notes.Valid {
			t.Notes = notes
		}

		fmt.Println("--- REQUEST ---")
		fmt.Printf("%s %s %s\n", t.RequestMethod.String, t.RequestURL.String, t.RequestHTTPVersion) // Use .String
		printHeaders(t.RequestHeaders.String)
		fmt.Println()
		printBody(t.RequestBody, "")

		fmt.Println("\n--- RESPONSE ---")
		if t.ResponseStatusCode > 0 || resHeadersStr.Valid {
			fmt.Printf("%s %d %s\n", t.ResponseHTTPVersion, t.ResponseStatusCode, t.ResponseReasonPhrase)
			printHeaders(t.ResponseHeaders)
			fmt.Println()
			printBody(t.ResponseBody, t.ResponseContentType)
		} else {
			fmt.Println("(No Response Recorded or Error Occurred)")
		}

		fmt.Println("\n--- METADATA ---")
		fmt.Printf("  Log ID:        %d\n", t.ID)
		if t.TargetID != nil {
			fmt.Printf("  Mapped Target: %d\n", *t.TargetID)
		} else {
			fmt.Println("  Mapped Target: (None)")
		}
		if !t.Timestamp.IsZero() {
			fmt.Printf("  Timestamp:     %s\n", t.Timestamp.Format(time.RFC3339))
		}
		fmt.Printf("  Duration:      %dms\n", t.DurationMs)
		fmt.Printf("  HTTPS:         %t\n", t.IsHTTPS)
		fmt.Printf("  Client IP:     %s\n", t.ClientIP)
		fmt.Printf("  Server IP:     %s\n", t.ServerIP)
		fmt.Printf("  Notes:         %s\n", t.Notes.String)

		fmt.Println("---")
		logger.Info("Successfully retrieved details for traffic log ID %d", logID)
	},
}

// --- Analyze Command ---
var trafficAnalyzeCmd = &cobra.Command{
	Use:   "analyze [log_id]",
	Short: "Analyze response body of a traffic log entry",
	Long: `Fetches the response body for a given traffic log ID and runs an analysis tool
(currently 'jsluice' for JavaScript) to extract interesting information like URLs, paths, secrets.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logIDStr := args[0]
		logger.Info("Executing 'traffic analyze' command for log ID: %s using tool: %s", logIDStr, trafficAnalyzeTool)

		logID, err := strconv.ParseInt(logIDStr, 10, 64)
		if err != nil {
			logger.Error("Invalid log_id provided: %s. Must be an integer.", logIDStr)
			fmt.Fprintf(os.Stderr, "Error: Invalid log_id '%s'. Must be an integer.\n", logIDStr)
			os.Exit(1)
		}

		var resBodyBytes []byte
		var contentType sql.NullString
		query := `SELECT response_body, response_content_type FROM http_traffic_log WHERE id = ?`
		err = database.DB.QueryRow(query, logID).Scan(&resBodyBytes, &contentType)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fmt.Fprintf(os.Stderr, "Error: Traffic log entry with ID %d not found.\n", logID)
			} else {
				logger.Error("traffic analyze: Error querying log ID %d: %v", logID, err)
				fmt.Fprintf(os.Stderr, "Error retrieving log entry %d from database.\n", logID)
			}
			os.Exit(1)
		}

		if len(resBodyBytes) == 0 {
			fmt.Printf("Log entry %d has an empty response body. Nothing to analyze.\n", logID)
			return
		}

		isJS := false
		if contentType.Valid {
			ctLower := strings.ToLower(contentType.String)
			if strings.Contains(ctLower, "javascript") || strings.Contains(ctLower, "ecmascript") {
				isJS = true
			}
		}

		if !isJS {
			fmt.Fprintf(os.Stderr, "Warning: Log entry %d response Content-Type ('%s') does not appear to be JavaScript. Analysis might yield unexpected results or fail.\n", logID, contentType.String)
		}

		var results map[string][]string
		var analysisErr error

		switch strings.ToLower(trafficAnalyzeTool) {
		case "jsluice":
			results, analysisErr = core.AnalyzeJSContent(resBodyBytes, logID)
		default:
			fmt.Fprintf(os.Stderr, "Error: Unknown analysis tool '%s'. Supported tools: jsluice\n", trafficAnalyzeTool)
			os.Exit(1)
		}

		if analysisErr != nil {
			fmt.Fprintf(os.Stderr, "Analysis using '%s' failed or found nothing for log ID %d: %v\n", trafficAnalyzeTool, logID, analysisErr)
			os.Exit(1)
		}

		fmt.Printf("--- Analysis Results for Log ID %d (Tool: %s) ---\n", logID, trafficAnalyzeTool)
		if len(results) == 0 {
			fmt.Println("(No specific items extracted by the tool)")
		} else {
			for category, items := range results {
				fmt.Printf("\n[%s]\n", category)
				if len(items) == 0 {
					fmt.Println("  (None found)")
				} else {
					for _, item := range items {
						fmt.Printf("  - %s\n", item)
					}
				}
			}
		}
		fmt.Println("--- End Analysis Results ---")

	},
}

// --- Purge Command ---
var trafficPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge unmapped and unanalyzed traffic log entries",
	Long: `Deletes entries from the http_traffic_log table that meet the following criteria:
  1. Their 'target_id' is NULL (meaning they are not mapped to any target).
  2. Their 'id' does not appear in the 'http_traffic_log_id' column of the 'analysis_results' table (meaning no analysis output is associated with them).
This command will ask for confirmation before proceeding unless the --force flag is used.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("Executing 'traffic purge' command")

		if !trafficPurgeForce {
			fmt.Print("WARNING: This will permanently delete unmapped and unanalyzed traffic log entries.\nAre you sure you want to continue? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(input)) != "yes" {
				fmt.Println("Purge operation cancelled.")
				return
			}
		}

		query := `
            DELETE FROM http_traffic_log
            WHERE target_id IS NULL 
            AND id NOT IN (
                SELECT DISTINCT http_traffic_log_id 
                FROM analysis_results 
                WHERE http_traffic_log_id IS NOT NULL
            )
        `
		logger.Debug("Executing purge query: %s", query)

		result, err := database.DB.Exec(query)
		if err != nil {
			logger.Error("Failed to execute purge command: %v", err)
			fmt.Fprintln(os.Stderr, "Error purging traffic log entries.")
			os.Exit(1)
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			logger.Error("Failed to get rows affected by purge: %v", err)
			fmt.Fprintln(os.Stderr, "Purge executed, but could not determine number of rows affected.")
		} else {
			fmt.Printf("Successfully purged %d unmapped and unanalyzed traffic log entries.\n", rowsAffected)
			logger.Info("Purged %d traffic log entries.", rowsAffected)
		}
	},
}

// --- Init Function ---

func init() {
	// List command flags
	trafficListCmd.Flags().StringVarP(&trafficListDomain, "domain", "d", "", "Filter traffic by requests containing this domain/host string")
	trafficListCmd.Flags().StringVarP(&trafficListRegex, "regex", "r", "", "Filter traffic by URL matching this Go regex pattern (applied after DB query)")
	trafficListCmd.Flags().StringVarP(&trafficListRegexField, "regex-field", "f", "url", "Field to apply regex filter on (url, req_headers, req_body, res_headers, res_body)")
	trafficListCmd.Flags().IntVarP(&trafficListStatusCode, "status-code", "s", 0, "Filter traffic by HTTP response status code (e.g., 200, 404)")
	trafficListCmd.Flags().IntVarP(&trafficListLimit, "limit", "l", 30, "Number of results per page")
	trafficListCmd.Flags().IntVarP(&trafficListPage, "page", "p", 1, "Page number to retrieve")

	// Map command flags
	trafficMapCmd.Flags().Int64VarP(&trafficMapTargetID, "target-id", "t", 0, "ID of the target to map the log entry to (uses current target if not specified)")

	// List Mapped command flags
	trafficListMappedCmd.Flags().Int64VarP(&trafficMapTargetID, "target-id", "t", 0, "ID of the target to list mapped entries for (uses current target if not specified)")

	// Analyze command flags
	trafficAnalyzeCmd.Flags().StringVar(&trafficAnalyzeTool, "tool", "jsluice", "Analysis tool to use (currently supports 'jsluice')")

	// Purge command flags
	trafficPurgeCmd.Flags().BoolVarP(&trafficPurgeForce, "force", "", false, "Skip confirmation before purging records")

	// Add subcommands to the base traffic command
	trafficCmd.AddCommand(trafficListCmd)
	trafficCmd.AddCommand(trafficMapCmd)
	trafficCmd.AddCommand(trafficListMappedCmd)
	trafficCmd.AddCommand(trafficGetCmd)
	trafficCmd.AddCommand(trafficAnalyzeCmd)
	trafficCmd.AddCommand(trafficPurgeCmd)

	// Add the base traffic command to the root command
	rootCmd.AddCommand(trafficCmd)
}
