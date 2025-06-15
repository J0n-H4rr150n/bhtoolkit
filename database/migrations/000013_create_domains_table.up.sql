CREATE TABLE domains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    target_id INTEGER NOT NULL,
    domain_name TEXT NOT NULL,
    source TEXT, -- How it was discovered (e.g., 'subfinder', 'manual', 'proxy')
    is_in_scope BOOLEAN DEFAULT FALSE,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (target_id) REFERENCES targets(id) ON DELETE CASCADE,
    UNIQUE (target_id, domain_name) -- Ensure domain_name is unique per target
);

CREATE INDEX idx_domains_target_id ON domains(target_id);
CREATE INDEX idx_domains_domain_name ON domains(domain_name);
CREATE TRIGGER update_domains_updated_at
AFTER UPDATE ON domains
FOR EACH ROW
BEGIN
    UPDATE domains SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

