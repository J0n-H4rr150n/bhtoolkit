-- /home/aibot/code/toolkit/database/migrations/000002_add_analytics_last_fetched_at_to_synack_targets.up.sql
ALTER TABLE synack_targets ADD COLUMN analytics_last_fetched_at DATETIME;
