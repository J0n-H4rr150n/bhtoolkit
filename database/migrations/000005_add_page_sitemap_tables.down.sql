-- Drop the 'page_http_logs' junction table first as it has foreign keys
-- referencing the 'pages' table.
DROP TABLE IF EXISTS page_http_logs;

-- Drop the index on the 'pages' table.
-- Note: SQLite automatically drops indexes when the table is dropped,
-- but explicitly dropping it is good practice for clarity and other DB systems.
DROP INDEX IF EXISTS idx_pages_target_id;

-- Drop the 'pages' table.
DROP TABLE IF EXISTS pages;
