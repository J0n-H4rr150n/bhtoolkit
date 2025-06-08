-- database/migrations/000001_initial_schema.up.sql

CREATE TABLE IF NOT EXISTS platforms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    platform_id INTEGER NOT NULL,
    slug TEXT UNIQUE,
    codename TEXT NOT NULL,
    link TEXT NOT NULL,
    notes TEXT,
    FOREIGN KEY(platform_id) REFERENCES platforms(id) ON DELETE CASCADE,
    UNIQUE (platform_id, codename)
);

CREATE TABLE IF NOT EXISTS scope_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    item_type TEXT NOT NULL CHECK(item_type IN ('domain', 'subdomain', 'ip_address', 'cidr', 'url_path')),
    pattern TEXT NOT NULL,
    is_in_scope BOOLEAN NOT NULL,
    is_wildcard BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT,
    FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, item_type, pattern, is_in_scope)
);

CREATE TABLE IF NOT EXISTS http_traffic_log (
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
    FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS web_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    title TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(target_id, url),
    FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS api_endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    method TEXT NOT NULL,
    path_pattern TEXT NOT NULL,
    description TEXT,
    parameters_info TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(target_id, method, path_pattern),
    FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS page_api_relationships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    web_page_id INTEGER NOT NULL,
    api_endpoint_id INTEGER NOT NULL,
    trigger_info TEXT,
    UNIQUE(web_page_id, api_endpoint_id),
    FOREIGN KEY(web_page_id) REFERENCES web_pages(id) ON DELETE CASCADE,
    FOREIGN KEY(api_endpoint_id) REFERENCES api_endpoints(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS analysis_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    http_log_id INTEGER NOT NULL,
    analysis_type TEXT NOT NULL,
    result_data TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(http_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS target_checklist_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    item_text TEXT NOT NULL,
    item_command_text TEXT,
    notes TEXT,
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (target_id, item_text),
    FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS synack_targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_target_id_str TEXT UNIQUE NOT NULL,
    codename TEXT,
    organization_id TEXT,
    activated_at TEXT,
    name TEXT,
    category TEXT,
    outage_windows_raw TEXT,
    vulnerability_discovery_raw TEXT,
    collaboration_criteria_raw TEXT,
    raw_json_details TEXT,
    first_seen_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'new',
    is_active BOOLEAN DEFAULT TRUE,
    deactivated_at DATETIME,
    notes TEXT,
    analytics_last_fetched_at DATETIME -- This column was added by a later migration, now part of initial
);

CREATE TABLE IF NOT EXISTS discovered_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    http_traffic_log_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    source_type TEXT NOT NULL,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (http_traffic_log_id, url, source_type),
    FOREIGN KEY(target_id) REFERENCES targets(id) ON DELETE SET NULL,
    FOREIGN KEY(http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS synack_target_analytics_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_target_db_id INTEGER NOT NULL,
    category_name TEXT NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY(synack_target_db_id) REFERENCES synack_targets(id) ON DELETE CASCADE,
    UNIQUE (synack_target_db_id, category_name)
);

CREATE TABLE IF NOT EXISTS synack_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_target_db_id INTEGER NOT NULL,
    synack_finding_id TEXT NOT NULL,
    title TEXT,
    category_name TEXT,
    severity TEXT,
    status TEXT,
    amount_paid REAL DEFAULT 0.0,
    vulnerability_url TEXT,
    reported_at DATETIME,
    closed_at DATETIME,
    raw_json_details TEXT,
    last_updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(synack_target_db_id) REFERENCES synack_targets(id) ON DELETE CASCADE,
    UNIQUE (synack_target_db_id, synack_finding_id)
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY NOT NULL,
    value TEXT,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS checklist_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT
);

CREATE TABLE IF NOT EXISTS checklist_template_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id INTEGER NOT NULL,
    item_text TEXT NOT NULL,
    item_command_text TEXT,
    notes TEXT,
    display_order INTEGER DEFAULT 0,
    UNIQUE (template_id, display_order),
    FOREIGN KEY(template_id) REFERENCES checklist_templates(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS target_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER,
    title TEXT NOT NULL,
    description TEXT,
    payload TEXT,
    severity TEXT,
    status TEXT NOT NULL DEFAULT 'Open',
    cvss_score REAL,
    cwe_id INTEGER,
    finding_references TEXT, -- This is the final name after the rename
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_target_findings_target_id ON target_findings(target_id);
CREATE INDEX IF NOT EXISTS idx_target_findings_severity ON target_findings(severity);
CREATE INDEX IF NOT EXISTS idx_target_findings_status ON target_findings(status);

CREATE TRIGGER IF NOT EXISTS update_target_finding_updated_at
AFTER UPDATE ON target_findings
FOR EACH ROW
BEGIN
    UPDATE target_findings SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

CREATE TABLE IF NOT EXISTS sitemap_manual_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER UNIQUE,
    folder_path TEXT NOT NULL,
    request_method TEXT NOT NULL,
    request_path TEXT NOT NULL,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_sitemap_manual_entries_target_id ON sitemap_manual_entries(target_id);
CREATE INDEX IF NOT EXISTS idx_sitemap_manual_entries_folder_path ON sitemap_manual_entries(folder_path);

CREATE TRIGGER IF NOT EXISTS update_sitemap_manual_entry_updated_at
AFTER UPDATE ON sitemap_manual_entries
FOR EACH ROW
BEGIN
    UPDATE sitemap_manual_entries SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- Initial data seeding for checklist_templates
INSERT OR IGNORE INTO checklist_templates (name, description) VALUES
('Basic Web Application Scan', 'A fundamental checklist for initial web application assessments.'),
('API Security Top 10 (OWASP)', 'Checklist based on OWASP API Security Top 10 vulnerabilities.'),
('Tool Commands', 'Commands for various tools.'),
('Tool Commands with Proxy', 'Commands for various tools with proxy support.');

