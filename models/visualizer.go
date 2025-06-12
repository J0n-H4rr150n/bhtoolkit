package models

import "database/sql"

// GraphNode represents a node in a graph visualization.
type GraphNode struct {
	ID    string `json:"id"`             // Unique identifier for the node
	Label string `json:"label"`          // Text displayed for the node
	Type  string `json:"type,omitempty"` // e.g., "folder", "endpoint", "page", "url" - used for styling
	// Data can hold any additional properties specific to the node type
	Data map[string]interface{} `json:"data,omitempty"`
}

// GraphEdge represents an edge connecting two nodes in a graph.
type GraphEdge struct {
	Source string `json:"source"`          // ID of the source node
	Target string `json:"target"`          // ID of the target node
	Label  string `json:"label,omitempty"` // Optional label for the edge (e.g., HTTP method)
	// Data can hold any additional properties specific to the edge
	Data map[string]interface{} `json:"data,omitempty"`
}

// GraphData is the container for nodes and edges that will be sent to the frontend.
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// DistinctPageLogURL is a helper struct used internally when processing Page Sitemap data.
// It helps in identifying unique URLs visited within a page context.
type DistinctPageLogURL struct {
	URL              string
	Method           string
	HTTPTrafficLogID sql.NullInt64 // To link back to an example log entry
}
