-- database/migrations/000001_initial_schema.down.sql

DROP TRIGGER IF EXISTS update_sitemap_manual_entry_updated_at;
DROP INDEX IF EXISTS idx_sitemap_manual_entries_folder_path;
DROP INDEX IF EXISTS idx_sitemap_manual_entries_target_id;
DROP TABLE IF EXISTS sitemap_manual_entries;

DROP TRIGGER IF EXISTS update_target_finding_updated_at;
DROP INDEX IF EXISTS idx_target_findings_status;
DROP INDEX IF EXISTS idx_target_findings_severity;
DROP INDEX IF EXISTS idx_target_findings_target_id;
DROP TABLE IF EXISTS target_findings;

DROP TABLE IF EXISTS notes;

DROP TABLE IF EXISTS checklist_template_items;
DROP TABLE IF EXISTS checklist_templates;

DROP TABLE IF EXISTS app_settings;

DROP TABLE IF EXISTS synack_findings;
DROP TABLE IF EXISTS synack_target_analytics_categories;
DROP TABLE IF EXISTS discovered_urls;
DROP TABLE IF EXISTS synack_targets;

DROP TABLE IF EXISTS target_checklist_items;
DROP TABLE IF EXISTS analysis_results;
DROP TABLE IF EXISTS page_api_relationships;
DROP TABLE IF EXISTS api_endpoints;
DROP TABLE IF EXISTS web_pages;

DROP TABLE IF EXISTS http_traffic_log;
DROP TABLE IF EXISTS scope_rules;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS platforms;
