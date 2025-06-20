CREATE TABLE IF NOT EXISTS tag_associations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id INTEGER NOT NULL,
    item_id INTEGER NOT NULL, -- ID of the item being tagged (e.g., http_traffic_log.id, domain.id)
    item_type TEXT NOT NULL, -- Type of the item (e.g., 'httplog', 'domain', 'finding', 'note')
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE,
    UNIQUE (tag_id, item_id, item_type) -- Prevent duplicate tag associations for the same item
);

CREATE TRIGGER IF NOT EXISTS tag_associations_updated_at
AFTER UPDATE ON tag_associations FOR EACH ROW
BEGIN UPDATE tag_associations SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id; END;