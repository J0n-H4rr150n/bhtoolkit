-- Remove the added columns and index
-- Note: SQLite doesn't directly support DROP COLUMN in older versions easily with ALTER TABLE.
-- For simplicity here, we'll assume a version that supports it or that you'd handle this
-- by creating a new table, copying data, and dropping the old one if needed.
-- However, the most common way for a simple rollback is to just drop the columns.
-- If your SQLite version is older, you might need a more complex downgrade script.

-- Drop the index first
DROP INDEX IF EXISTS idx_http_traffic_log_page_sitemap_id;

-- Recreate table without the columns (Safer for older SQLite, but more complex)
-- OR, if your SQLite version supports it (3.35.0+):
ALTER TABLE http_traffic_log DROP COLUMN page_sitemap_id;
ALTER TABLE http_traffic_log DROP COLUMN log_source;

-- If you need the older SQLite compatible way:
/*
PRAGMA foreign_keys=off;

CREATE TABLE http_traffic_log_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    request_method TEXT,
    request_url TEXT,
    request_http_version TEXT,
    request_headers TEXT, -- JSON
    request_body BLOB,
    response_status_code INTEGER,
    response_reason_phrase TEXT,
    response_http_version TEXT,
    response_headers TEXT, -- JSON
    response_body BLOB,
    response_content_type TEXT,
    response_body_size INTEGER,
    duration_ms INTEGER,
    client_ip TEXT,
    server_ip TEXT,
    is_https BOOLEAN,
    is_page_candidate BOOLEAN DEFAULT FALSE,
    notes TEXT,
    request_full_url_with_fragment TEXT,
    is_favorite BOOLEAN DEFAULT FALSE,
    source_modifier_task_id INTEGER,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (source_modifier_task_id) REFERENCES modifier_tasks(id) ON DELETE SET NULL
);

INSERT INTO http_traffic_log_new (
    id, target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
    response_status_code, response_reason_phrase, response_http_version, response_headers, response_body,
    response_content_type, response_body_size, duration_ms, client_ip, server_ip, is_https, is_page_candidate, notes,
    request_full_url_with_fragment, is_favorite, source_modifier_task_id
) SELECT
    id, target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
    response_status_code, response_reason_phrase, response_http_version, response_headers, response_body,
    response_content_type, response_body_size, duration_ms, client_ip, server_ip, is_https, is_page_candidate, notes,
    request_full_url_with_fragment, is_favorite, source_modifier_task_id
FROM http_traffic_log;

DROP TABLE http_traffic_log;
ALTER TABLE http_traffic_log_new RENAME TO http_traffic_log;

PRAGMA foreign_keys=on;
*/
