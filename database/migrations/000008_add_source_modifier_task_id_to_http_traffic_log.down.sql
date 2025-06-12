PRAGMA foreign_keys=off;

CREATE TABLE http_traffic_log_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    request_method TEXT,
    request_url TEXT,
    request_http_version TEXT,
    request_headers TEXT,
    request_body BLOB,
    response_status_code INTEGER,
    response_reason_phrase TEXT,
    response_http_version TEXT,
    response_headers TEXT,
    response_body BLOB,
    response_content_type TEXT,
    response_body_size INTEGER,
    duration_ms INTEGER,
    client_ip TEXT,
    server_ip TEXT,
    is_https BOOLEAN,
    is_page_candidate BOOLEAN DEFAULT FALSE,
    notes TEXT,
    is_favorite BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE SET NULL
);

INSERT INTO http_traffic_log_new (
    id, target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
    response_status_code, response_reason_phrase, response_http_version, response_headers, response_body,
    response_content_type, response_body_size, duration_ms, client_ip, server_ip, is_https,
    is_page_candidate, notes, is_favorite
) SELECT
    id, target_id, timestamp, request_method, request_url, request_http_version, request_headers, request_body,
    response_status_code, response_reason_phrase, response_http_version, response_headers, response_body,
    response_content_type, response_body_size, duration_ms, client_ip, server_ip, is_https,
    is_page_candidate, notes, is_favorite
FROM http_traffic_log;

DROP TABLE http_traffic_log;
ALTER TABLE http_traffic_log_new RENAME TO http_traffic_log;

PRAGMA foreign_keys=on;
