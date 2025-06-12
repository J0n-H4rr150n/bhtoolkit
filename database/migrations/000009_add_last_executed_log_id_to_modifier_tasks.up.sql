ALTER TABLE modifier_tasks ADD COLUMN last_executed_log_id INTEGER NULL;
CREATE INDEX IF NOT EXISTS idx_modifier_tasks_last_executed_log_id ON modifier_tasks (last_executed_log_id);
