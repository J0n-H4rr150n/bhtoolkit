ALTER TABLE modifier_tasks ADD COLUMN source_log_id INTEGER NULL;
ALTER TABLE modifier_tasks ADD COLUMN source_param_url_id INTEGER NULL;

CREATE INDEX IF NOT EXISTS idx_modifier_tasks_source_log_id ON modifier_tasks (source_log_id);
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_source_param_url_id ON modifier_tasks (source_param_url_id);