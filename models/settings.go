package models

// ProxyExclusionRule defines the structure for a global proxy exclusion rule.
type ProxyExclusionRule struct {
	ID          string `json:"id"`          // Unique ID for the rule (e.g., UUID, could be generated on client)
	RuleType    string `json:"rule_type"`   // e.g., "file_extension", "url_regex", "domain"
	Pattern     string `json:"pattern"`     // The actual pattern to match (e.g., ".css", "google-analytics\.com", "ads.example.com")
	Description string `json:"description"` // Optional description for the rule
	IsEnabled   bool   `json:"is_enabled"`  // Whether the rule is active
}

// ProxyExclusionRulesKey is the key used in app_settings for storing global proxy exclusion rules.
const ProxyExclusionRulesKey = "proxy_exclusion_rules"

// UISettingsKey is the key for general UI settings
const UISettingsKey = "ui_settings"

// CurrentTargetIDKey is the key used in app_settings for the current target ID.
const CurrentTargetIDKey = "current_target_id"

// CustomHTTPHeadersKey is the key used in app_settings for custom HTTP headers.
const CustomHTTPHeadersKey = "custom_http_headers"

// TableColumnWidthsKey is the key used in app_settings for table column widths.
const TableColumnWidthsKey = "table_column_widths"
