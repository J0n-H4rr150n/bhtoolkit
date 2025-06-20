package models

import (
	"database/sql"
	"time"
)

type HTTPTrafficLog struct {
	ID                         int64          `json:"id" readOnly:"true"`
	TargetID                   *int64         `json:"target_id,omitempty"`
	Timestamp                  time.Time      `json:"timestamp" readOnly:"true"`
	RequestMethod              sql.NullString `json:"request_method" example:"GET"`
	RequestURL                 sql.NullString `json:"request_url" example:"https://example.com/api/data?id=123"`
	RequestHTTPVersion         sql.NullString `json:"request_http_version,omitempty" example:"HTTP/1.1"`
	RequestHeaders             sql.NullString `json:"request_headers,omitempty" example:"{\"Content-Type\":[\"application/json\"]}"`
	RequestBody                []byte         `json:"request_body,omitempty"`
	ResponseStatusCode         int            `json:"response_status_code,omitempty" example:"200"`
	ResponseReasonPhrase       sql.NullString `json:"response_reason_phrase,omitempty" example:"OK"`
	ResponseHTTPVersion        sql.NullString `json:"response_http_version,omitempty" example:"HTTP/1.1"`
	ResponseHeaders            sql.NullString `json:"response_headers,omitempty" example:"{\"Content-Type\":[\"application/json\"]}"`
	ResponseBody               []byte         `json:"response_body,omitempty"`
	ResponseContentType        sql.NullString `json:"response_content_type,omitempty" example:"application/json"`
	ResponseBodySize           int64          `json:"response_body_size,omitempty" example:"1024"`
	DurationMs                 int64          `json:"duration_ms,omitempty" example:"150"`
	ClientIP                   sql.NullString `json:"client_ip,omitempty" example:"192.168.1.100"`
	ServerIP                   sql.NullString `json:"server_ip,omitempty" example:"203.0.113.45"`
	IsHTTPS                    bool           `json:"is_https" example:"true"`
	IsPageCandidate            bool           `json:"is_page_candidate" example:"false"`
	Notes                      sql.NullString `json:"notes,omitempty"`
	RequestFullURLWithFragment sql.NullString `json:"request_full_url_with_fragment,omitempty"`
	IsFavorite                 bool           `json:"is_favorite"`
	SourceModifierTaskID       sql.NullInt64  `json:"source_modifier_task_id,omitempty"` // Changed to sql.NullInt64 to match DB
	LogSource                  sql.NullString `json:"log_source,omitempty"`
	PageSitemapID              sql.NullInt64  `json:"page_sitemap_id,omitempty"`
	PageSitemapName            sql.NullString `json:"page_sitemap_name,omitempty"`
	AssociatedFindings         []FindingLink  `json:"associated_findings,omitempty"` // Already added in a previous step
	Tags                       []Tag          `json:"tags,omitempty"`                // For associating tags with log entries
}
