ALTER TABLE http_traffic_log ADD COLUMN source_modifier_task_id INTEGER NULL;
CREATE INDEX IF NOT EXISTS idx_http_traffic_log_source_modifier_task_id ON http_traffic_log (source_modifier_task_id);
