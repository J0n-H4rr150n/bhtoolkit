ALTER TABLE modifier_tasks ADD COLUMN display_order INTEGER DEFAULT 0 NOT NULL;

-- Initialize display_order for existing tasks.
-- This example orders them by their creation date, then ID.
-- You might want a different initial order.
UPDATE modifier_tasks
SET display_order = (
    SELECT COUNT(*)
    FROM modifier_tasks m2
    WHERE (m2.created_at < modifier_tasks.created_at) OR (m2.created_at = modifier_tasks.created_at AND m2.id <= modifier_tasks.id)
);
