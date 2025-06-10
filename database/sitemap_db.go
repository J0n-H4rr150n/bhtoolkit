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

func ensurePathNode(segments []string, rootNodes *[]*models.SitemapTreeNode, nodesMap map[string]*models.SitemapTreeNode) *models.SitemapTreeNode {
	currentPathAgg := ""
	var parentNode *models.SitemapTreeNode

	if len(segments) == 0 || (len(segments) == 1 && segments[0] == "") {
		segments = []string{"/"}
	}

	for _, segmentName := range segments {
		prospectivePath := currentPathAgg + "/" + segmentName
		if currentPathAgg == "/" && segmentName == "/" {
			prospectivePath = "/"
		} else if currentPathAgg == "" && segmentName == "/" {
			prospectivePath = "/"
		} else if currentPathAgg == "/" {
			prospectivePath = "/" + segmentName
		}
		prospectivePath = normalizeNodePath(prospectivePath)

		node, exists := nodesMap[prospectivePath]
		if !exists {
			node = &models.SitemapTreeNode{
				Name:     segmentName,
				FullPath: prospectivePath,
				Children: []*models.SitemapTreeNode{},
			}
			nodesMap[prospectivePath] = node
			if parentNode != nil {
				parentNode.Children = append(parentNode.Children, node)
			} else {
				if prospectivePath == "/" {
					isRootNodePresentInList := false
					for _, rn := range *rootNodes {
						if rn.FullPath == "/" {
							isRootNodePresentInList = true
							break
						}
					}
					if !isRootNodePresentInList {
						*rootNodes = append(*rootNodes, node)
					}
				} else if rootNode, rootExists := nodesMap["/"]; rootExists {
					rootNode.Children = append(rootNode.Children, node)
				} else { // Should not happen if "/" is pre-created
					*rootNodes = append(*rootNodes, node)
				}
			}
		}
		parentNode = node
		currentPathAgg = prospectivePath
	}
	return parentNode
}

// BuildSitemapTree constructs the sitemap tree from log entries and manual entries.
func BuildSitemapTree(logEntries []LogEntryForSitemap, manualEntries []models.SitemapManualEntry) []*models.SitemapTreeNode {
	rootNodes := []*models.SitemapTreeNode{}
	nodesMap := make(map[string]*models.SitemapTreeNode)

	if _, ok := nodesMap["/"]; !ok {
		rootNode := &models.SitemapTreeNode{Name: "/", FullPath: "/", Children: []*models.SitemapTreeNode{}}
		nodesMap["/"] = rootNode
		rootNodes = append(rootNodes, rootNode)
	}

	for _, logEntry := range logEntries {
		parsedURL, err := url.Parse(logEntry.RequestURL)
		if err != nil {
			logger.Info("BuildSitemapTree: Could not parse URL '%s' from log ID %d: %v", logEntry.RequestURL, logEntry.ID, err) // Changed Warn to Info
			continue
		}
		// Construct the path for the tree node (directory structure)
		treePath := parsedURL.Path
		if treePath == "" {
			treePath = "/"
		}
		// Construct the full path for the endpoint, including query string
		endpointPath := parsedURL.Path
		if parsedURL.RawQuery != "" {
			endpointPath += "?" + parsedURL.RawQuery
		}

		segments := strings.Split(strings.Trim(treePath, "/"), "/")
		if treePath == "/" {
			segments = []string{"/"}
		}

		leafNode := ensurePathNode(segments, &rootNodes, nodesMap)
		if leafNode != nil {
			endpoint := models.SitemapEndpoint{
				HTTPTrafficLogID: logEntry.ID,
				Method:           logEntry.RequestMethod,
				Path:             endpointPath,                // Use the path with query string
				StatusCode:       logEntry.ResponseStatusCode, // Direct assignment, already sql.NullInt64
				ResponseSize:     logEntry.ResponseBodySize,   // Direct assignment, already sql.NullInt64
				IsFavorite:       logEntry.IsFavorite,         // Direct assignment, already sql.NullBool
			}
			leafNode.Endpoints = append(leafNode.Endpoints, endpoint)
		}
	}

	for _, manualEntry := range manualEntries {
		folderPath := normalizeNodePath(manualEntry.FolderPath)
		segments := strings.Split(strings.Trim(folderPath, "/"), "/")
		if folderPath == "/" {
			segments = []string{"/"}
		}

		folderNode := ensurePathNode(segments, &rootNodes, nodesMap)
		if folderNode != nil {
			endpoint := models.SitemapEndpoint{
				Method:           manualEntry.RequestMethod,
				Path:             manualEntry.RequestPath, // This should ideally include query for manual entries too
				IsManuallyAdded:  true,
				ManualEntryID:    manualEntry.ID,
				ManualEntryNotes: manualEntry.Notes,
				HTTPTrafficLogID: manualEntry.HTTPTrafficLogID.Int64,
			}
			folderNode.Endpoints = append(folderNode.Endpoints, endpoint)
		}
	}

	sortTreeNodes(rootNodes)
	return rootNodes
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
