-- Add the new column to store the full URL including fragment, if available
ALTER TABLE http_traffic_log ADD COLUMN request_full_url_with_fragment TEXT NULL;