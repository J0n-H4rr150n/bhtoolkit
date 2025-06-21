-- Triggers first
DROP TRIGGER IF EXISTS update_sitemap_manual_entry_updated_at;
DROP TRIGGER IF EXISTS update_target_finding_updated_at;
DROP TRIGGER IF EXISTS update_note_updated_at;
DROP TRIGGER IF EXISTS update_checklist_template_item_updated_at;
DROP TRIGGER IF EXISTS update_target_checklist_item_updated_at;
DROP TRIGGER IF EXISTS pages_updated_at;
DROP TRIGGER IF EXISTS parameterized_urls_updated_at;
DROP TRIGGER IF EXISTS modifier_tasks_updated_at;
DROP TRIGGER IF EXISTS domains_updated_at;
DROP TRIGGER IF EXISTS tags_updated_at;
DROP TRIGGER IF EXISTS tag_associations_updated_at;
DROP TRIGGER IF EXISTS vulnerability_types_updated_at;
DROP TRIGGER IF EXISTS synack_missions_updated_at;

-- Indexes next
DROP INDEX IF EXISTS idx_sitemap_manual_entries_folder_path;
DROP INDEX IF EXISTS idx_sitemap_manual_entries_target_id;
DROP INDEX IF EXISTS idx_target_findings_status;
DROP INDEX IF EXISTS idx_target_findings_severity;
DROP INDEX IF EXISTS idx_target_findings_target_id;
DROP INDEX IF EXISTS idx_http_traffic_log_target_id;
DROP INDEX IF EXISTS idx_http_traffic_log_url;
DROP INDEX IF EXISTS idx_http_traffic_log_page_sitemap_id;
DROP INDEX IF EXISTS idx_pages_target_id;

-- Tables in reverse order of creation
DROP TABLE IF EXISTS page_api_relationships;
DROP TABLE IF EXISTS api_endpoints;
DROP TABLE IF EXISTS web_pages;
DROP TABLE IF EXISTS analysis_results;
DROP TABLE IF EXISTS target_checklist_items;
DROP TABLE IF EXISTS discovered_urls;
DROP TABLE IF EXISTS synack_missions;
DROP TABLE IF EXISTS synack_findings;
DROP TABLE IF EXISTS synack_target_analytics_categories;
DROP TABLE IF EXISTS synack_targets;
DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS checklist_template_items;
DROP TABLE IF EXISTS checklist_templates;
DROP TABLE IF EXISTS notes;
DROP TABLE IF EXISTS sitemap_manual_entries;
DROP TABLE IF EXISTS target_findings;
DROP TABLE IF EXISTS vulnerability_types;
DROP TABLE IF EXISTS tag_associations;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS modifier_tasks;
DROP TABLE IF EXISTS parameterized_urls;
DROP TABLE IF EXISTS page_http_logs;
DROP TABLE IF EXISTS http_traffic_log;
DROP TABLE IF EXISTS pages;
DROP TABLE IF EXISTS scope_rules;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS platforms;