PRAGMA foreign_keys=off;

CREATE TABLE modifier_tasks_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    name TEXT NOT NULL,
    base_request_url TEXT NOT NULL,
    base_request_method TEXT NOT NULL,
    base_request_headers TEXT,
    base_request_body TEXT,
    display_order INTEGER NOT NULL DEFAULT 0,
    source_log_id INTEGER NULL,
    source_param_url_id INTEGER NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (source_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    FOREIGN KEY (source_param_url_id) REFERENCES parameterized_urls(id) ON DELETE SET NULL
);

INSERT INTO modifier_tasks_new (id, target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, source_log_id, source_param_url_id, created_at, updated_at)
SELECT id, target_id, name, base_request_url, base_request_method, base_request_headers, base_request_body, display_order, source_log_id, source_param_url_id, created_at, updated_at
FROM modifier_tasks;

DROP TABLE modifier_tasks;
ALTER TABLE modifier_tasks_new RENAME TO modifier_tasks;

PRAGMA foreign_keys=on;
