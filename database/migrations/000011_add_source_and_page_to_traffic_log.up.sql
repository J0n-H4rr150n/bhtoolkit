-- Add log_source column with a default value
ALTER TABLE http_traffic_log ADD COLUMN log_source TEXT DEFAULT 'mitmproxy';

-- Add page_sitemap_id column as a foreign key to the pages table
-- This allows associating a log entry directly with a recorded page sitemap
ALTER TABLE http_traffic_log ADD COLUMN page_sitemap_id INTEGER NULL REFERENCES pages(id) ON DELETE SET NULL;

-- Optionally, create an index for faster lookups if you query by page_sitemap_id often
CREATE INDEX IF NOT EXISTS idx_http_traffic_log_page_sitemap_id ON http_traffic_log (page_sitemap_id);
