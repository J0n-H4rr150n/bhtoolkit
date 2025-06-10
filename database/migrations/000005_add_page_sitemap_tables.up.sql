-- Create the 'pages' table to store information about recorded page loads.
CREATE TABLE pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    start_timestamp DATETIME NOT NULL,
    end_timestamp DATETIME, -- Nullable until recording is stopped
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE
);

-- Create an index on target_id for faster querying of pages.
CREATE INDEX idx_pages_target_id ON pages(target_id);

-- Create the 'page_http_logs' junction table to link pages
-- to their associated HTTP log entries.
CREATE TABLE page_http_logs (
    page_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER NOT NULL,
    PRIMARY KEY (page_id, http_traffic_log_id),
    FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE
);
