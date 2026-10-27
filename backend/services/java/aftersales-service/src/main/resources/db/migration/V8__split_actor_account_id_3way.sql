-- V8__split_actor_account_id_3way.sql
ALTER TABLE aftersale_audit_logs
  ADD COLUMN actor_admin_account_id BIGINT,
  ADD COLUMN actor_buyer_id BIGINT,
  ADD COLUMN actor_store_id BIGINT;

UPDATE aftersale_audit_logs SET actor_admin_account_id = actor_account_id WHERE actor_type = '8404';
UPDATE aftersale_audit_logs SET actor_buyer_id = actor_account_id WHERE actor_type = '8401';
UPDATE aftersale_audit_logs SET actor_store_id = actor_account_id WHERE actor_type = '8402';
-- actor_type = '8403' (SYSTEM) rows: all 3 new columns stay NULL, matching the existing
-- actor_account_id NULL convention for SYSTEM rows (confirmed 5999/5999 rows already NULL).

ALTER TABLE aftersale_audit_logs DROP COLUMN actor_account_id;

COMMENT ON COLUMN aftersale_audit_logs.actor_admin_account_id IS 'Real login account_id, populated only when actor_type=ADMIN(8404).';
COMMENT ON COLUMN aftersale_audit_logs.actor_buyer_id IS 'Resolved buyer_id, populated only when actor_type=BUYER(8401).';
COMMENT ON COLUMN aftersale_audit_logs.actor_store_id IS 'Resolved store_id, populated only when actor_type=SELLER(8402).';
