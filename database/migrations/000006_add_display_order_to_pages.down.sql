-- Drop the index first
DROP INDEX IF EXISTS idx_pages_target_id_display_order;

-- Remove the display_order column from the pages table
-- Note: SQLite's ALTER TABLE DROP COLUMN is available in version 3.35.0+.
-- If using an older version, you'd need a more complex procedure (create new table, copy data, drop old, rename new).
-- Assuming a modern SQLite version for simplicity.
ALTER TABLE pages DROP COLUMN display_order;
