-- Platforms Table
CREATE TABLE IF NOT EXISTS platforms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- Targets Table
CREATE TABLE IF NOT EXISTS targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    platform_id INTEGER NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    codename TEXT NOT NULL,
    link TEXT NOT NULL,
    notes TEXT,
    FOREIGN KEY (platform_id) REFERENCES platforms(id) ON DELETE CASCADE,
    UNIQUE (platform_id, codename)
);

-- Scope Rules Table
CREATE TABLE IF NOT EXISTS scope_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    pattern TEXT NOT NULL,
    is_in_scope BOOLEAN NOT NULL,
    is_wildcard BOOLEAN NOT NULL,
    description TEXT,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, item_type, pattern, is_in_scope)
);

-- Pages Table (for Page Sitemap)
CREATE TABLE IF NOT EXISTS pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    start_timestamp DATETIME NOT NULL,
    end_timestamp DATETIME,
    display_order INTEGER DEFAULT 0 NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE
);
CREATE TRIGGER IF NOT EXISTS pages_updated_at
AFTER UPDATE ON pages FOR EACH ROW
BEGIN UPDATE pages SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;
CREATE INDEX IF NOT EXISTS idx_pages_target_id ON pages(target_id);

-- HTTP Traffic Log Table
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
    request_full_url_with_fragment TEXT,
    is_favorite BOOLEAN DEFAULT FALSE NOT NULL,
    source_modifier_task_id INTEGER,
    log_source TEXT,
    page_sitemap_id INTEGER,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE SET NULL,
    FOREIGN KEY (page_sitemap_id) REFERENCES pages(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_http_traffic_log_target_id ON http_traffic_log(target_id);
CREATE INDEX IF NOT EXISTS idx_http_traffic_log_url ON http_traffic_log(request_url);
CREATE INDEX IF NOT EXISTS idx_http_traffic_log_page_sitemap_id ON http_traffic_log(page_sitemap_id);

-- Page HTTP Logs Junction Table
CREATE TABLE IF NOT EXISTS page_http_logs (
    page_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER NOT NULL,
    PRIMARY KEY (page_id, http_traffic_log_id),
    FOREIGN KEY (page_id) REFERENCES pages(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE
);

-- Parameterized URLs Table
CREATE TABLE IF NOT EXISTS parameterized_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    http_traffic_log_id INTEGER,
    request_method TEXT,
    request_path TEXT,
    param_keys TEXT NOT NULL,
    example_full_url TEXT,
    notes TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    hit_count INTEGER DEFAULT 1,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    UNIQUE (target_id, request_method, request_path, param_keys)
);
CREATE TRIGGER IF NOT EXISTS parameterized_urls_updated_at
AFTER UPDATE ON parameterized_urls FOR EACH ROW
BEGIN UPDATE parameterized_urls SET last_seen_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Modifier Tasks Table
CREATE TABLE IF NOT EXISTS modifier_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    name TEXT NOT NULL,
    base_request_method TEXT NOT NULL,
    base_request_url TEXT NOT NULL,
    base_request_headers TEXT,
    base_request_body TEXT,
    original_request_headers TEXT,
    original_request_body TEXT,
    original_response_headers TEXT,
    original_response_body TEXT,
    last_executed_log_id INTEGER,
    source_log_id INTEGER,
    source_param_url_id INTEGER,
    display_order INTEGER DEFAULT 0 NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (last_executed_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    FOREIGN KEY (source_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    FOREIGN KEY (source_param_url_id) REFERENCES parameterized_urls(id) ON DELETE SET NULL
);
CREATE TRIGGER IF NOT EXISTS modifier_tasks_updated_at
AFTER UPDATE ON modifier_tasks FOR EACH ROW
BEGIN UPDATE modifier_tasks SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Domains Table
CREATE TABLE IF NOT EXISTS domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    domain_name TEXT NOT NULL,
    source TEXT,
    is_in_scope BOOLEAN DEFAULT FALSE,
    is_wildcard_scope BOOLEAN DEFAULT FALSE,
    notes TEXT,
    is_favorite BOOLEAN DEFAULT FALSE NOT NULL,
    http_status_code INTEGER,
    http_content_length INTEGER,
    http_title TEXT,
    http_server TEXT,
    http_tech TEXT,
    httpx_full_json TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, domain_name)
);
CREATE TRIGGER IF NOT EXISTS domains_updated_at
AFTER UPDATE ON domains FOR EACH ROW
BEGIN UPDATE domains SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Tags Table
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER IF NOT EXISTS tags_updated_at
AFTER UPDATE ON tags FOR EACH ROW
BEGIN UPDATE tags SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Tag Associations Table
CREATE TABLE IF NOT EXISTS tag_associations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE (tag_id, item_id, item_type)
);
CREATE TRIGGER IF NOT EXISTS tag_associations_updated_at
AFTER UPDATE ON tag_associations FOR EACH ROW
BEGIN UPDATE tag_associations SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Vulnerability Types Table
CREATE TABLE IF NOT EXISTS vulnerability_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER IF NOT EXISTS vulnerability_types_updated_at
AFTER UPDATE ON vulnerability_types FOR EACH ROW
BEGIN UPDATE vulnerability_types SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Target Findings Table
CREATE TABLE IF NOT EXISTS target_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER,
    vulnerability_type_id INTEGER,
    title TEXT NOT NULL,
    summary TEXT,
    description TEXT,
    steps_to_reproduce TEXT,
    impact TEXT,
    recommendations TEXT,
    payload TEXT,
    severity TEXT,
    status TEXT NOT NULL,
    cvss_score REAL,
    cwe_id INTEGER,
    finding_references TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL,
    FOREIGN KEY (vulnerability_type_id) REFERENCES vulnerability_types(id) ON DELETE SET NULL
);
CREATE TRIGGER IF NOT EXISTS update_target_finding_updated_at
AFTER UPDATE ON target_findings FOR EACH ROW
BEGIN UPDATE target_findings SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;
CREATE INDEX IF NOT EXISTS idx_target_findings_target_id ON target_findings(target_id);
CREATE INDEX IF NOT EXISTS idx_target_findings_severity ON target_findings(severity);
CREATE INDEX IF NOT EXISTS idx_target_findings_status ON target_findings(status);

-- Sitemap Manual Entries Table
CREATE TABLE IF NOT EXISTS sitemap_manual_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    http_traffic_log_id INTEGER,
    folder_path TEXT NOT NULL,
    request_method TEXT NOT NULL,
    request_path TEXT NOT NULL,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE SET NULL
);
CREATE TRIGGER IF NOT EXISTS update_sitemap_manual_entry_updated_at
AFTER UPDATE ON sitemap_manual_entries FOR EACH ROW
BEGIN UPDATE sitemap_manual_entries SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;
CREATE INDEX IF NOT EXISTS idx_sitemap_manual_entries_target_id ON sitemap_manual_entries(target_id);
CREATE INDEX IF NOT EXISTS idx_sitemap_manual_entries_folder_path ON sitemap_manual_entries(folder_path);

-- Notes Table
CREATE TABLE IF NOT EXISTS notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER IF NOT EXISTS update_note_updated_at
AFTER UPDATE ON notes FOR EACH ROW
BEGIN UPDATE notes SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Checklist Templates Table
CREATE TABLE IF NOT EXISTS checklist_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT
);

-- Checklist Template Items Table
CREATE TABLE IF NOT EXISTS checklist_template_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id INTEGER NOT NULL,
    item_text TEXT NOT NULL,
    item_command_text TEXT,
    notes TEXT,
    display_order INTEGER DEFAULT 0 NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (template_id) REFERENCES checklist_templates(id) ON DELETE CASCADE,
    UNIQUE (template_id, item_text)
);
CREATE TRIGGER IF NOT EXISTS update_checklist_template_item_updated_at
AFTER UPDATE ON checklist_template_items FOR EACH ROW
BEGIN UPDATE checklist_template_items SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- App Settings Table
CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT
);

-- Synack Targets Table
CREATE TABLE IF NOT EXISTS synack_targets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_target_id_str TEXT NOT NULL UNIQUE,
    codename TEXT,
    organization_id TEXT,
    activated_at TEXT,
    name TEXT,
    category TEXT,
    outage_windows_raw TEXT,
    vulnerability_discovery_raw TEXT,
    collaboration_criteria_raw TEXT,
    raw_json_details TEXT,
    first_seen_timestamp DATETIME,
    last_seen_timestamp DATETIME,
    status TEXT,
    is_active BOOLEAN,
    deactivated_at DATETIME,
    notes TEXT,
    analytics_last_fetched_at DATETIME
);

-- Synack Target Analytics Categories Table
CREATE TABLE IF NOT EXISTS synack_target_analytics_categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_target_db_id INTEGER NOT NULL,
    category_name TEXT NOT NULL,
    count INTEGER NOT NULL,
    FOREIGN KEY (synack_target_db_id) REFERENCES synack_targets(id) ON DELETE CASCADE,
    UNIQUE (synack_target_db_id, category_name)
);

-- Synack Findings Table
CREATE TABLE IF NOT EXISTS synack_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_target_db_id INTEGER NOT NULL,
    synack_finding_id TEXT NOT NULL,
    title TEXT,
    category_name TEXT,
    severity TEXT,
    status TEXT,
    amount_paid REAL,
    vulnerability_url TEXT,
    reported_at DATETIME,
    closed_at DATETIME,
    raw_json_details TEXT,
    last_updated_at DATETIME,
    FOREIGN KEY (synack_target_db_id) REFERENCES synack_targets(id) ON DELETE CASCADE,
    UNIQUE (synack_target_db_id, synack_finding_id)
);

-- Synack Missions Table
CREATE TABLE IF NOT EXISTS synack_missions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    synack_task_id TEXT NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    campaign_name TEXT,
    organization_uid TEXT,
    listing_uid TEXT,
    listing_codename TEXT,
    campaign_uid TEXT,
    payout_amount REAL,
    payout_currency TEXT,
    status TEXT NOT NULL DEFAULT 'UNKNOWN',
    claimed_by_toolkit_at DATETIME,
    synack_api_claimed_at DATETIME,
    synack_api_expires_at DATETIME,
    raw_mission_details_json TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TRIGGER IF NOT EXISTS synack_missions_updated_at
AFTER UPDATE ON synack_missions FOR EACH ROW
BEGIN UPDATE synack_missions SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Discovered URLs Table
CREATE TABLE IF NOT EXISTS discovered_urls (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER,
    http_traffic_log_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    source_type TEXT NOT NULL,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE SET NULL,
    FOREIGN KEY (http_traffic_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE,
    UNIQUE (http_traffic_log_id, url, source_type)
);

-- Target Checklist Items Table
CREATE TABLE IF NOT EXISTS target_checklist_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    item_text TEXT NOT NULL,
    item_command_text TEXT,
    notes TEXT,
    is_completed BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, item_text)
);
CREATE TRIGGER IF NOT EXISTS update_target_checklist_item_updated_at
AFTER UPDATE ON target_checklist_items FOR EACH ROW
BEGIN UPDATE target_checklist_items SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;

-- Analysis Results Table
CREATE TABLE IF NOT EXISTS analysis_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    http_log_id INTEGER NOT NULL,
    analysis_type TEXT NOT NULL,
    result_data TEXT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (http_log_id) REFERENCES http_traffic_log(id) ON DELETE CASCADE,
    UNIQUE (http_log_id, analysis_type)
);

-- Web Pages Table
CREATE TABLE IF NOT EXISTS web_pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    title TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, url)
);

-- API Endpoints Table
CREATE TABLE IF NOT EXISTS api_endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    method TEXT NOT NULL,
    path_pattern TEXT NOT NULL,
    description TEXT,
    parameters_info TEXT,
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, method, path_pattern)
);

-- Page API Relationships Table
CREATE TABLE IF NOT EXISTS page_api_relationships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    web_page_id INTEGER NOT NULL,
    api_endpoint_id INTEGER NOT NULL,
    trigger_info TEXT,
    FOREIGN KEY (web_page_id) REFERENCES web_pages(id) ON DELETE CASCADE,
    FOREIGN KEY (api_endpoint_id) REFERENCES api_endpoints(id) ON DELETE CASCADE,
    UNIQUE (web_page_id, api_endpoint_id)
);