-- database/migrations/00000X_add_parameterized_urls_table.up.sql

CREATE TABLE IF NOT EXISTS parameterized_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER NOT NULL, -- Store one example log ID
    request_method TEXT NOT NULL,
    request_path TEXT NOT NULL,          -- URL path without query string
    param_keys TEXT NOT NULL,            -- Sorted, comma-separated list of parameter keys
    example_full_url TEXT NOT NULL,      -- Full URL from the example log entry
    notes TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP, -- When an instance was last processed
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE,
    UNIQUE (target_id, request_method, request_path, param_keys)
);

CREATE INDEX IF NOT EXISTS idx_parameterized_urls_target_id ON parameterized_urls(target_id);
CREATE INDEX IF NOT EXISTS idx_parameterized_urls_request_path ON parameterized_urls(request_path);
CREATE INDEX IF NOT EXISTS idx_parameterized_urls_param_keys ON parameterized_urls(param_keys);

CREATE TRIGGER IF NOT EXISTS update_parameterized_urls_last_seen_at
AFTER UPDATE ON parameterized_urls
FOR EACH ROW
WHEN NEW.last_seen_at <= OLD.last_seen_at -- Only update if not explicitly set to an older date
BEGIN
    UPDATE parameterized_urls SET last_seen_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;
