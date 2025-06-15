-- Assuming a modern SQLite version that supports DROP COLUMN.
-- If not, you'd use the table recreation strategy.
ALTER TABLE domains DROP COLUMN is_favorite;