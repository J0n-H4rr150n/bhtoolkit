-- database/migrations/00000Y_add_modifier_tables.up.sql

CREATE TABLE IF NOT EXISTS modifier_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    parameterized_url_id INTEGER,
    original_log_id INTEGER, -- Could be from http_traffic_log
    name TEXT NOT NULL,
    base_request_method TEXT NOT NULL,
    base_request_url TEXT NOT NULL,
    base_request_headers TEXT, -- Store as JSON string
    base_request_body TEXT,    -- Store as Base64 encoded string
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE SET NULL,
    FOREIGN KEY (parameterized_url_id) REFERENCES parameterized_urls(id) ON DELETE SET NULL,
    FOREIGN KEY (original_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_modifier_tasks_target_id ON modifier_tasks(target_id);
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_name ON modifier_tasks(name);

CREATE TRIGGER IF NOT EXISTS update_modifier_tasks_updated_at
AFTER UPDATE ON modifier_tasks
FOR EACH ROW
BEGIN
    UPDATE modifier_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- Placeholder for versions table, will be expanded later
CREATE TABLE IF NOT EXISTS modifier_task_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    modifier_task_id INTEGER NOT NULL,
    -- Add fields for modified request (method, url, headers, body)
    -- Add fields for response (status, headers, body, duration)
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (modifier_task_id) REFERENCES modifier_tasks(id) ON DELETE CASCADE
);
