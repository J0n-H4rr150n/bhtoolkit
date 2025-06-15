DROP TRIGGER IF EXISTS update_domains_updated_at;
DROP INDEX IF EXISTS idx_domains_domain_name;
DROP INDEX IF EXISTS idx_domains_target_id;
DROP TABLE IF EXISTS domains;
