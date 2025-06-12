package database

import (
	"database/sql"
	"fmt"
	"strings"
	"toolkit/logger"
	"toolkit/models"
)

// GetSitemapGraphData generates graph data (nodes and edges) from sitemap data for visualization.
// It builds a hierarchical graph based on the sitemap tree structure.
func GetSitemapGraphData(targetID int64) (*models.GraphData, error) {
	// Fetch data needed to build the sitemap tree
	logEntries, err := GetLogEntriesForSitemapGeneration(targetID)
	if err != nil {
		// Error is already logged inside GetLogEntriesForSitemapGeneration
		return nil, fmt.Errorf("failed to retrieve log entries for sitemap graph: %w", err)
	}

	manualEntries, err := GetSitemapManualEntriesByTargetID(targetID)
	if err != nil {
		// Error is already logged inside GetSitemapManualEntriesByTargetID
		return nil, fmt.Errorf("failed to retrieve manual sitemap entries for sitemap graph: %w", err)
	}

	// Build the sitemap tree structure
	sitemapTree := BuildSitemapTree(logEntries, manualEntries)

	// Prepare maps to store nodes and edges, using maps to easily check for duplicates
	nodes := make(map[string]models.GraphNode)
	edges := []models.GraphEdge{}

	// Add a root node if the tree is not empty, representing the base path "/"
	rootID := "/"
	nodes[rootID] = models.GraphNode{
		ID:    rootID,
		Label: "/",
		Type:  "folder", // Type for styling
		Data:  map[string]interface{}{"fullPath": rootID},
	}

	// Recursive function to traverse the sitemap tree and build graph nodes and edges
	var traverse func(nodes []*models.SitemapTreeNode, parentID string)
	traverse = func(treeNodes []*models.SitemapTreeNode, parentID string) {
		for _, node := range treeNodes {
			// Create a node ID for the current folder/path segment.
			// Using the full path ensures uniqueness for folders.
			nodeID := node.FullPath

			// Add node for the current path segment/folder if it doesn't exist
			if _, exists := nodes[nodeID]; !exists {
				nodes[nodeID] = models.GraphNode{
					ID:    nodeID,
					Label: node.Name, // Use the segment name as the node label
					Type:  "folder",
					Data:  map[string]interface{}{"fullPath": node.FullPath},
				}
			}

			// Add an edge from the parent node to the current folder node
			if parentID != "" { // Don't add an edge from an empty parent (should only happen for root children)
				edges = append(edges, models.GraphEdge{
					Source: parentID,
					Target: nodeID,
					Label:  "", // No label needed for hierarchical folder edges
				})
			}

			// Add nodes and edges for endpoints found directly under this path
			for _, endpoint := range node.Endpoints {
				// Create a unique ID for the endpoint node.
				// A combination of method and path is needed for uniqueness,
				// as multiple logs might hit the same endpoint.
				// Using a simple combination for now, might need hashing for very long paths.
				// Ensure the ID is safe for frontend use (no special characters that break selectors).
				endpointID := fmt.Sprintf("endpoint-%s-%s", endpoint.Method, strings.ReplaceAll(endpoint.Path, "/", "_"))
				// Add log ID or manual entry ID to data for linking
				endpointData := map[string]interface{}{
					"method":          endpoint.Method,
					"path":            endpoint.Path,
					"isManuallyAdded": endpoint.IsManuallyAdded,
				}
				if endpoint.HTTPTrafficLogID.Valid {
					endpointData["logId"] = endpoint.HTTPTrafficLogID.Int64
				} else if endpoint.ManualEntryID.Valid { // If not from a direct log, check if it's a manual entry
					endpointData["manualEntryId"] = endpoint.ManualEntryID.Int64
				}
				if endpoint.StatusCode.Valid {
					endpointData["statusCode"] = endpoint.StatusCode.Int64
				}
				if endpoint.ResponseSize.Valid {
					endpointData["responseSize"] = endpoint.ResponseSize.Int64
				}
				if endpoint.IsFavorite.Valid {
					endpointData["isFavorite"] = endpoint.IsFavorite.Bool
				}

				endpointLabel := fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)

				// Add node for the endpoint if it doesn't exist
				if _, exists := nodes[endpointID]; !exists {
					nodes[endpointID] = models.GraphNode{
						ID:    endpointID,
						Label: endpointLabel,
						Type:  "endpoint", // Type for styling
						Data:  endpointData,
					}
				}

				// Add an edge from the folder node to the endpoint node
				edges = append(edges, models.GraphEdge{
					Source: nodeID,
					Target: endpointID,
					Label:  endpoint.Method, // Use the HTTP method as the edge label
					Data: map[string]interface{}{
						"method": endpoint.Method,
					},
				})
			}

			// Recursively traverse the children nodes
			if len(node.Children) > 0 {
				traverse(node.Children, nodeID)
			}
		}
	}

	// Start the traversal from the top-level nodes of the sitemap tree
	traverse(sitemapTree, rootID)

	// Convert the map of nodes into a slice
	nodeSlice := make([]models.GraphNode, 0, len(nodes))
	for _, node := range nodes {
		nodeSlice = append(nodeSlice, node)
	}

	logger.Info("Generated Sitemap Graph: %d nodes, %d edges for target %d", len(nodeSlice), len(edges), targetID)

	return &models.GraphData{
		Nodes: nodeSlice,
		Edges: edges,
	}, nil
}

// GetPageSitemapGraphData generates graph data (nodes and edges) from page sitemap data for visualization.
// It visualizes the relationship between recorded pages and the distinct URLs visited within those pages.
func GetPageSitemapGraphData(targetID int64) (*models.GraphData, error) { // Added targetID parameter
	// Fetch all recorded pages for the target
	pages, err := GetPagesForTarget(targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve pages for page sitemap graph: %w", err)
	}

	return generatePageGraph(pages)
}

// GetDistinctURLsForPage is a helper function to fetch distinct URLs (Method + Path)
// from the http_traffic_log table for a given page_id.
func GetDistinctURLsForPage(pageID int64) ([]models.DistinctPageLogURL, error) {
	// Ensure the page_http_logs table exists and correctly links pages to http_traffic_log entries.
	// It selects the method, URL, and the MIN(id) of the log entries for each unique URL within that page,
	// effectively giving one example log ID for each distinct request.
	query := `
        SELECT htl.request_method, htl.request_url, MIN(htl.id) as min_id
        FROM http_traffic_log htl
        JOIN page_http_logs phl ON htl.id = phl.http_traffic_log_id
        WHERE phl.page_id = ?
        GROUP BY htl.request_method, htl.request_url
		ORDER BY htl.request_url, htl.request_method;
    `

	rows, err := DB.Query(query, pageID)
	if err != nil {
		return nil, fmt.Errorf("querying distinct URLs for page %d: %w", pageID, err)
	}
	defer rows.Close()

	var distinctURLs []models.DistinctPageLogURL
	for rows.Next() {
		var du models.DistinctPageLogURL
		var logID sql.NullInt64 // Use sql.NullInt64 for scanning the ID
		if err := rows.Scan(&du.Method, &du.URL, &logID); err != nil {
			return nil, fmt.Errorf("scanning distinct URL for page %d: %w", pageID, err)
		}
		du.HTTPTrafficLogID = logID // Assign the scanned ID
		distinctURLs = append(distinctURLs, du)
	}

	return distinctURLs, rows.Err()
}

// generatePageGraph constructs the graph data from pages and their associated logs.
func generatePageGraph(pages []models.Page) (*models.GraphData, error) {
	nodes := make(map[string]models.GraphNode)
	var edges []models.GraphEdge

	for _, page := range pages {
		pageID := fmt.Sprintf("page-%d", page.ID)
		nodes[pageID] = models.GraphNode{
			ID:    pageID,
			Label: page.Name,
			Type:  "page",
			Data: map[string]interface{}{
				"pageId":      page.ID,
				"description": page.Description.String,
				"startTime":   page.StartTimestamp, // Corrected field name
				"endTime":     page.EndTimestamp,   // Corrected field name
			},
		}

		distinctURLs, err := GetDistinctURLsForPage(page.ID)
		if err != nil {
			logger.Error("generatePageGraph: Error fetching distinct URLs for page %d: %v", page.ID, err)
			continue // Skip this page if URLs can't be fetched, but continue with others
		}

		for _, u := range distinctURLs {
			urlNodeID := fmt.Sprintf("url-%s-%s", u.Method, strings.ReplaceAll(u.URL, "/", "_"))
			if _, exists := nodes[urlNodeID]; !exists {
				nodes[urlNodeID] = models.GraphNode{
					ID:    urlNodeID,
					Label: fmt.Sprintf("%s %s", u.Method, u.URL),
					Type:  "url",
					Data:  map[string]interface{}{"method": u.Method, "path": u.URL, "exampleLogId": u.HTTPTrafficLogID.Int64},
				}
			}
			edges = append(edges, models.GraphEdge{Source: pageID, Target: urlNodeID, Label: u.Method})
		}
	}

	nodeSlice := make([]models.GraphNode, 0, len(nodes))
	for _, node := range nodes {
		nodeSlice = append(nodeSlice, node)
	}
	// Ensure Nodes and Edges are never nil, even if empty
	if nodeSlice == nil {
		nodeSlice = []models.GraphNode{}
	}
	if edges == nil {
		edges = []models.GraphEdge{}
	}

	return &models.GraphData{Nodes: nodeSlice, Edges: edges}, nil // Return GraphData with Nodes and Edges
}

// Ensure GetLogEntriesForSitemapGeneration, GetSitemapManualEntriesByTargetID, BuildSitemapTree,
// GetPagesForTarget are available in the database package or imported from other database files.
// Assuming they are available in this package for now.
// GetPagesForTarget needs to be implemented if it's not already.
// GetLogsForPage is likely already implemented for Page Sitemap view, but GetDistinctURLsForPage is new.
