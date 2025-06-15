ALTER TABLE domains ADD COLUMN is_wildcard_scope BOOLEAN DEFAULT FALSE;
-- We can also add an index if we frequently query by this, but it might be rare. For now, let's omit it.
