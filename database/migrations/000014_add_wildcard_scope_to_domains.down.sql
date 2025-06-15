-- SQLite does not directly support DROP COLUMN in older versions without workarounds.
-- However, modern SQLite (3.35.0+) supports it.
-- Assuming a modern SQLite version, the command would be:
-- ALTER TABLE domains DROP COLUMN is_wildcard_scope;
-- For broader compatibility, the common workaround is to recreate the table.
-- Since this is a simple boolean column with a default, dropping it is generally safe if supported.
-- If your SQLite version is older, you'd need the recreate table strategy.
-- For now, providing the direct command assuming modern SQLite:
ALTER TABLE domains DROP COLUMN is_wildcard_scope;

