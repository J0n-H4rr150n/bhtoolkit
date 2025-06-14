CREATE TABLE IF NOT EXISTS modifier_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    name TEXT NOT NULL,
    base_request_method TEXT NOT NULL,
    base_request_url TEXT NOT NULL,
    base_request_headers TEXT, -- Store as JSON string
    base_request_body TEXT,    -- Store as Base64 encoded string
    last_executed_log_id INTEGER,
    source_log_id INTEGER,
    source_param_url_id INTEGER,
    display_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE SET NULL,
    FOREIGN KEY (last_executed_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    FOREIGN KEY (source_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    FOREIGN KEY (source_param_url_id) REFERENCES parameterized_urls(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_modifier_tasks_target_id ON modifier_tasks(target_id);
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_name ON modifier_tasks(name);
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_last_executed_log_id ON modifier_tasks (last_executed_log_id);
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_source_log_id ON modifier_tasks (source_log_id);
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_source_param_url_id ON modifier_tasks (source_param_url_id);

CREATE TRIGGER IF NOT EXISTS update_modifier_tasks_updated_at
AFTER UPDATE ON modifier_tasks
FOR EACH ROW
BEGIN
    UPDATE modifier_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;
