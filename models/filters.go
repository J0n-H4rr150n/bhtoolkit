package models

// ProxyLogFilters defines parameters for filtering proxy log queries.
type ProxyLogFilters struct {
	TargetID            int64  `json:"target_id"`
	Page                int    `json:"page"`
	Limit               int    `json:"limit"`
	SortBy              string `json:"sort_by"`
	SortOrder           string `json:"sort_order"`
	FilterFavoritesOnly bool   `json:"favorites_only"`
	FilterMethod        string `json:"method,omitempty"`
	FilterStatus        string `json:"status,omitempty"`
	FilterContentType   string `json:"type,omitempty"`
	FilterSearchText    string `json:"search,omitempty"`
	AnalysisType        string `json:"analysis_type,omitempty"` // For specific analyses like "params"
}

// ParameterizedURLFilters defines parameters for filtering parameterized URL queries.
type ParameterizedURLFilters struct {
	TargetID        int64  `json:"target_id"`
	Page            int    `json:"page"`
	Limit           int    `json:"limit"`
	SortBy          string `json:"sort_by"`
	SortOrder       string `json:"sort_order"`
	RequestMethod   string `json:"request_method,omitempty"`
	PathSearch      string `json:"path_search,omitempty"`
	ParamKeysSearch string `json:"param_keys_search,omitempty"`
}

// Add other filter structs here as needed for different parts of your application.

// PaginatedResponse is a generic structure for paginated API responses.
type PaginatedResponse struct {
	Page         int         `json:"page"`
	Limit        int         `json:"limit"`
	TotalRecords int         `json:"total_records"`
	TotalPages   int         `json:"total_pages"`
	Records      interface{} `json:"records"` // Can hold any type of record slice
}
