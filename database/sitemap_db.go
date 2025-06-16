package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"

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

// LogEntryForSitemap holds data fetched from http_traffic_log for sitemap generation.
type LogEntryForSitemap struct {
	ID                 int64
	RequestMethod      string
	RequestURL         string
	ResponseStatusCode sql.NullInt64
	ResponseBodySize   sql.NullInt64
	IsFavorite         sql.NullBool
}

// GetLogEntriesForSitemapGeneration fetches relevant data from http_traffic_log for a target.
func GetLogEntriesForSitemapGeneration(targetID int64) ([]LogEntryForSitemap, error) {
	query := `
		SELECT id, request_method, request_url, response_status_code, response_body_size, is_favorite
		FROM http_traffic_log
		WHERE target_id = ?
		ORDER BY request_url ASC, request_method ASC
	`

	rows, err := DB.Query(query, targetID)
	if err != nil {
		logger.Error("GetLogEntriesForSitemapGeneration: Error querying http_traffic_log for target %d: %v", targetID, err)
		return nil, fmt.Errorf("querying http_traffic_log: %w", err)
	}
	defer rows.Close()

	var entries []LogEntryForSitemap
	for rows.Next() {
		var entry LogEntryForSitemap
		if err := rows.Scan(
			&entry.ID,
			&entry.RequestMethod,
			&entry.RequestURL,
			&entry.ResponseStatusCode,
			&entry.ResponseBodySize,
			&entry.IsFavorite,
		); err != nil {
			logger.Error("GetLogEntriesForSitemapGeneration: Error scanning log entry: %v", err)
			continue
		}
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		logger.Error("GetLogEntriesForSitemapGeneration: Error after iterating rows: %v", err)
		return nil, fmt.Errorf("iterating log entry rows: %w", err)
	}

	return entries, nil
}

func normalizeNodePath(path string) string {
	if path == "" {
		return "/"
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// ensurePathNodeUnderHost creates or retrieves the path nodes under a given hostNode.
// hostNode: The parent host node.
// relativePath: The path string relative to the host (e.g., "/api/users").
// nodesMap: Global map of all nodes, keys are absolute full paths (e.g., "host.com/api/users").
// hostname: The name of the host, for constructing absolute paths.
func ensurePathNodeUnderHost(hostNode *models.SitemapTreeNode, relativePath string, nodesMap map[string]*models.SitemapTreeNode, hostname string) *models.SitemapTreeNode {
	// Normalize relativePath: ensure it starts with a slash if not empty
	if relativePath != "/" && !strings.HasPrefix(relativePath, "/") {
		relativePath = "/" + relativePath
	}
	if relativePath == "" { // Treat empty path as root of the host
		relativePath = "/"
	}

	// If the relative path is just "/", the hostNode itself is the target leaf node for endpoints at the root of the host.
	if relativePath == "/" {
		return hostNode
	}

	segments := strings.Split(strings.Trim(relativePath, "/"), "/")
	currentNode := hostNode

	currentAbsolutePathPrefix := hostname // Path prefix for keys in nodesMap starts with the hostname

	// Iterate over path segments to build/traverse the tree under the host
	for _, segmentName := range segments {
		if segmentName == "" {
			continue // Should not happen with strings.Trim and then Split, but good for safety
		}

		// Construct the absolute full path for this segment node
		childNodeAbsolutePath := currentAbsolutePathPrefix + "/" + segmentName
		// Normalize to remove trailing slashes for map key consistency, except for root "/"
		// This normalization might not be strictly necessary if segmentName is always clean.
		// childNodeAbsolutePath = normalizeNodePath(childNodeAbsolutePath) // normalizeNodePath might be too aggressive here.

		childNode, exists := nodesMap[childNodeAbsolutePath]
		if !exists {
			childNode = &models.SitemapTreeNode{
				Name:     segmentName,
				FullPath: childNodeAbsolutePath, // Store absolute path
				Type:     "folder",              // Path segments are folders
				Children: []*models.SitemapTreeNode{},
			}
			nodesMap[childNodeAbsolutePath] = childNode
			currentNode.Children = append(currentNode.Children, childNode)
		}
		currentNode = childNode
		currentAbsolutePathPrefix = childNodeAbsolutePath // Update prefix for the next segment
	}
	return currentNode // This is the leaf path node
}

// BuildSitemapTree constructs the sitemap tree from log entries and manual entries. Returns the top-level nodes.
func BuildSitemapTree(logEntries []LogEntryForSitemap, manualEntries []models.SitemapManualEntry) []*models.SitemapTreeNode {
	hostMap := make(map[string]*models.SitemapTreeNode) // Stores host nodes: "example.com" -> *SitemapTreeNode
	nodesMap := make(map[string]*models.SitemapTreeNode)

	// Process Log Entries
	for _, logEntry := range logEntries {
		parsedURL, err := url.Parse(logEntry.RequestURL)
		if err != nil {
			logger.Info("BuildSitemapTree: Could not parse URL '%s' from log ID %d: %v", logEntry.RequestURL, logEntry.ID, err) // Changed Warn to Info
			continue
		}

		hostname := parsedURL.Hostname()
		if hostname == "" {
			logger.Info("BuildSitemapTree: Could not extract hostname from URL '%s' (Log ID %d)", logEntry.RequestURL, logEntry.ID)
			continue
		}

		// Get or create host node
		hostNode, hostExists := nodesMap[hostname]
		if !hostExists {
			hostNode = &models.SitemapTreeNode{
				Name:     hostname,
				FullPath: hostname,
				Type:     "host", // Mark as host
				Children: []*models.SitemapTreeNode{},
			}
			nodesMap[hostname] = hostNode
			hostMap[hostname] = hostNode // Keep track of top-level hosts
		}

		// Path under the host
		pathUnderHost := parsedURL.Path

		// Ensure path nodes under this host exist
		// The returned leafNode is where the endpoint will be attached.
		// If pathUnderHost is "/", leafNode will be hostNode itself.
		leafNode := ensurePathNodeUnderHost(hostNode, pathUnderHost, nodesMap, hostname)

		// Construct the full path for the endpoint, including query string for display
		endpointPath := parsedURL.Path
		if parsedURL.RawQuery != "" {
			endpointPath += "?" + parsedURL.RawQuery
		}

		if leafNode != nil {
			endpoint := models.SitemapEndpoint{
				HTTPTrafficLogID: sql.NullInt64{Int64: logEntry.ID, Valid: logEntry.ID != 0}, // Assuming ID 0 is invalid/not set
				Method:           logEntry.RequestMethod,
				Path:             endpointPath,
				StatusCode:       logEntry.ResponseStatusCode,
				ResponseSize:     logEntry.ResponseBodySize,
				IsFavorite:       logEntry.IsFavorite,
				IsManuallyAdded:  false,
				ManualEntryID:    sql.NullInt64{},
			}
			leafNode.Endpoints = append(leafNode.Endpoints, endpoint)
		}
	}

	// Process Manual Entries
	for _, manualEntry := range manualEntries {
		var entryHostname string
		var entryPathUnderHost string

		// Attempt to determine hostname for the manual entry
		if manualEntry.HTTPTrafficLogID.Valid {
			// If linked to a log, derive hostname from that log's URL
			var originalLogURL string
			err := DB.QueryRow("SELECT request_url FROM http_traffic_log WHERE id = ?", manualEntry.HTTPTrafficLogID.Int64).Scan(&originalLogURL)
			if err == nil {
				if parsedURL, parseErr := url.Parse(originalLogURL); parseErr == nil {
					entryHostname = parsedURL.Hostname()
				}
			}
			if entryHostname == "" {
				logger.Info("BuildSitemapTree: Could not determine host from linked log ID %d for manual entry ID %d. Skipping.", manualEntry.HTTPTrafficLogID.Int64, manualEntry.ID)
				continue
			}
		} else {
			// If no linked log, try to parse FolderPath as "host/path" or assume a default/first host.
			// This part is heuristic and might need refinement based on how FolderPath is structured for manual entries.
			// For now, let's assume FolderPath might be like "example.com/admin" or just "/admin"
			// If it's just "/admin", it's ambiguous. We'll try to attach it to the first available host.
			// A better approach would be for manual entries to explicitly store a hostname or be relative to one.
			parts := strings.SplitN(strings.Trim(manualEntry.FolderPath, "/"), "/", 2)
			if len(parts) > 0 && strings.Contains(parts[0], ".") { // Heuristic: if first part looks like a domain
				entryHostname = parts[0]
				if len(parts) > 1 {
					entryPathUnderHost = "/" + parts[1]
				} else {
					entryPathUnderHost = "/"
				}
			} else { // Path doesn't start with a domain-like string
				if len(hostMap) > 0 {
					for hn := range hostMap { // Pick the first host found
						entryHostname = hn
						break
					}
				} else {
					logger.Info("BuildSitemapTree: No hosts available to attach manual entry ID %d (FolderPath: %s). Skipping.", manualEntry.ID, manualEntry.FolderPath)
					continue
				}
				entryPathUnderHost = manualEntry.FolderPath
			}
		}

		if entryHostname == "" {
			logger.Info("BuildSitemapTree: Could not determine hostname for manual entry ID %d (FolderPath: %s). Skipping.", manualEntry.ID, manualEntry.FolderPath)
			continue
		}

		hostNode, hostExists := nodesMap[entryHostname]
		if !hostExists {
			// This case should ideally not happen if log entries are processed first,
			// or if manual entries are always for known hosts.
			// If it can happen, create the host node here.
			hostNode = &models.SitemapTreeNode{Name: entryHostname, FullPath: entryHostname, Type: "host", Children: []*models.SitemapTreeNode{}}
			nodesMap[entryHostname] = hostNode
			hostMap[entryHostname] = hostNode
		}

		leafNode := ensurePathNodeUnderHost(hostNode, entryPathUnderHost, nodesMap, entryHostname)

		if leafNode != nil {
			endpoint := models.SitemapEndpoint{
				Method:           manualEntry.RequestMethod,
				Path:             manualEntry.RequestPath, // This is the endpoint path itself
				HTTPTrafficLogID: manualEntry.HTTPTrafficLogID,
				IsManuallyAdded:  true,
				ManualEntryID:    sql.NullInt64{Int64: manualEntry.ID, Valid: manualEntry.ID != 0},
				ManualEntryNotes: manualEntry.Notes,
				StatusCode:       sql.NullInt64{}, // Manual entries don't have these from logs
				ResponseSize:     sql.NullInt64{},
				IsFavorite:       sql.NullBool{}, // Manual entries don't have favorite status from logs
			}
			leafNode.Endpoints = append(leafNode.Endpoints, endpoint)
		}
	}

	// Convert hostMap to slice for return
	finalTree := make([]*models.SitemapTreeNode, 0, len(hostMap))
	for _, hostNode := range hostMap {
		finalTree = append(finalTree, hostNode)
	}

	// Sort hosts by name
	sort.Slice(finalTree, func(i, j int) bool {
		return finalTree[i].Name < finalTree[j].Name
	})

	// Sort children and endpoints within each host tree
	for _, hostNode := range finalTree {
		sortTreeNodes(hostNode.Children) // Sort children of each host
		// Also sort endpoints directly under the host node if any
		sort.Slice(hostNode.Endpoints, func(i, j int) bool {
			if hostNode.Endpoints[i].Method != hostNode.Endpoints[j].Method {
				return hostNode.Endpoints[i].Method < hostNode.Endpoints[j].Method
			}
			return hostNode.Endpoints[i].Path < hostNode.Endpoints[j].Path
		})
	}

	return finalTree
}

func sortTreeNodes(nodes []*models.SitemapTreeNode) {
	for _, node := range nodes {
		sort.Slice(node.Children, func(i, j int) bool { return node.Children[i].Name < node.Children[j].Name })
		sort.Slice(node.Endpoints, func(i, j int) bool {
			if node.Endpoints[i].Method != node.Endpoints[j].Method {
				return node.Endpoints[i].Method < node.Endpoints[j].Method
			}
			return node.Endpoints[i].Path < node.Endpoints[j].Path
		})
		if len(node.Children) > 0 {
			sortTreeNodes(node.Children)
		}
	}
}
