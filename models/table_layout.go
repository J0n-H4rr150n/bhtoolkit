package models

// ColumnConfig defines the structure for a single column's configuration within a table layout.
type ColumnConfig struct {
	Width   string `json:"width"`
	Visible bool   `json:"visible"`
}

// TableLayoutConfig defines the structure for a single table's layout configuration.
type TableLayoutConfig struct {
	Columns  map[string]ColumnConfig `json:"columns"`            // Maps column key to its config (width, visibility)
	PageSize int                     `json:"pageSize,omitempty"` // omitempty if pageSize is optional or not always present
}

// AllTableLayouts represents the structure for all table layouts.
// It's a map where the key is the table identifier (e.g., "proxyLogTable").
type AllTableLayouts map[string]TableLayoutConfig
