-- Add display_order column to the pages table
ALTER TABLE pages ADD COLUMN display_order INTEGER DEFAULT 0;

-- Optionally, you might want to initialize existing records.
-- This example sets display_order based on existing start_timestamp for a basic initial order.
-- For more complex scenarios, you might need a more sophisticated script or handle it in application logic.
UPDATE pages
SET display_order = (
    SELECT COUNT(*)
    FROM pages p2
    WHERE p2.target_id = pages.target_id AND p2.start_timestamp <= pages.start_timestamp
) - 1;

-- Create an index on display_order for faster sorting, possibly combined with target_id
CREATE INDEX idx_pages_target_id_display_order ON pages(target_id, display_order);
