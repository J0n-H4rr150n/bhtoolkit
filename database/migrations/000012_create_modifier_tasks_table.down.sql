-- /mnt/data_drive/code/bhtoolkit/database/migrations/000012_create_modifier_tasks_table.down.sql
DROP TRIGGER IF EXISTS update_modifier_tasks_updated_at;
DROP INDEX IF EXISTS idx_modifier_tasks_source_param_url_id;
DROP INDEX IF EXISTS idx_modifier_tasks_source_log_id;
DROP INDEX IF EXISTS idx_modifier_tasks_last_executed_log_id;
DROP INDEX IF EXISTS idx_modifier_tasks_name;
DROP INDEX IF EXISTS idx_modifier_tasks_target_id;
DROP TABLE IF EXISTS modifier_tasks;