ALTER TABLE domains ADD COLUMN http_status_code INTEGER;
ALTER TABLE domains ADD COLUMN http_content_length INTEGER;
ALTER TABLE domains ADD COLUMN http_title TEXT;
ALTER TABLE domains ADD COLUMN http_server TEXT;
ALTER TABLE domains ADD COLUMN http_tech TEXT;
ALTER TABLE domains ADD COLUMN httpx_full_json TEXT;