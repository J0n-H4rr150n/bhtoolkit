-- Revert adding item_command_text from target_checklist_items
-- SQLite does not support DROP COLUMN directly in older versions.
-- This typically involves creating a new table, copying data, dropping old, renaming new.
-- For simplicity in a development environment, this down migration might be a no-op
-- or you'd implement the full table recreation if strict rollback is needed.
-- For this example, we'll acknowledge the column would remain if rolling back.

-- PRAGMA foreign_keys=off;
-- CREATE TABLE target_checklist_items_new AS SELECT id, target_id, item_text, notes, is_completed, created_at, updated_at FROM target_checklist_items;
-- DROP TABLE target_checklist_items;
-- ALTER TABLE target_checklist_items_new RENAME TO target_checklist_items;
-- PRAGMA foreign_keys=on;
SELECT 'Column item_command_text would remain if this down migration is run without full table recreation.';