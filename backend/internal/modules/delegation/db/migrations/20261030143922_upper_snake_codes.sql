-- 儲存的代碼改為 UPPER_SNAKE（ADR 2026-09-25-codemeta-retirement D3）。順序：先放掉舊 CHECK 與依賴 status 的 partial index，
-- 再改既有列，最後以大寫值重建。
-- Drop constraints
ALTER TABLE "delegation"."admin_operation_audit_logs" DROP CONSTRAINT "admin_operation_audit_logs_origin_check", DROP CONSTRAINT "admin_operation_audit_logs_resource_type_check", DROP CONSTRAINT "admin_operation_audit_logs_risk_level_check";
ALTER TABLE "delegation"."delegated_operation_sessions" DROP CONSTRAINT "delegated_operation_sessions_reason_code_check", DROP CONSTRAINT "delegated_operation_sessions_revoked_check", DROP CONSTRAINT "delegated_operation_sessions_status_check";
ALTER TABLE "delegation"."seller_delegation_authorizations" DROP CONSTRAINT "seller_delegation_authorizations_global_check", DROP CONSTRAINT "seller_delegation_authorizations_source_check";
-- Drop indexes
DROP INDEX "delegation"."uidx_delegated_operation_sessions_active_actor";
DROP INDEX "delegation"."uidx_delegated_operation_sessions_active_store";
-- Rewrite existing rows
UPDATE "delegation"."delegated_operation_sessions" SET "status" = upper("status"), "reason_code" = upper("reason_code");
UPDATE "delegation"."admin_operation_audit_logs" SET "origin" = upper("origin"), "resource_type" = upper("resource_type"), "risk_level" = upper("risk_level");
UPDATE "delegation"."seller_delegation_authorizations" SET "source" = upper("source");
UPDATE "delegation"."delegation_authorization_audit_logs" SET "source" = upper("source");
-- Add constraints
ALTER TABLE "delegation"."admin_operation_audit_logs" ADD CONSTRAINT "admin_operation_audit_logs_origin_check" CHECK (origin = ANY (ARRAY['INTERACTIVE'::text, 'SCHEDULED'::text])), ADD CONSTRAINT "admin_operation_audit_logs_resource_type_check" CHECK (resource_type = ANY (ARRAY['STOREFRONT'::text, 'PRODUCT'::text, 'POST'::text, 'ORDER'::text])), ADD CONSTRAINT "admin_operation_audit_logs_risk_level_check" CHECK (risk_level = ANY (ARRAY['LOW'::text, 'MEDIUM'::text, 'HIGH'::text]));
ALTER TABLE "delegation"."delegated_operation_sessions" ADD CONSTRAINT "delegated_operation_sessions_reason_code_check" CHECK (reason_code = ANY (ARRAY['SELLER_REQUEST'::text, 'PLATFORM_MAINTENANCE'::text, 'CONTENT_GOVERNANCE'::text])), ADD CONSTRAINT "delegated_operation_sessions_revoked_check" CHECK ((status = 'REVOKED'::text) = (revoked_at IS NOT NULL)), ADD CONSTRAINT "delegated_operation_sessions_status_check" CHECK (status = ANY (ARRAY['ACTIVE'::text, 'REVOKED'::text]));
ALTER TABLE "delegation"."seller_delegation_authorizations" ADD CONSTRAINT "seller_delegation_authorizations_global_check" CHECK ((target_store_id IS NOT NULL) OR (source = 'PLATFORM_POLICY'::text)), ADD CONSTRAINT "seller_delegation_authorizations_source_check" CHECK (source = ANY (ARRAY['CONTRACT'::text, 'TICKET'::text, 'PLATFORM_POLICY'::text]));
-- Create indexes
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_active_actor" ON "delegation"."delegated_operation_sessions" ("actor_admin_id") WHERE ((status = 'ACTIVE'::text) AND (ended_at IS NULL));
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_active_store" ON "delegation"."delegated_operation_sessions" ("target_store_id") WHERE ((status = 'ACTIVE'::text) AND (ended_at IS NULL));
