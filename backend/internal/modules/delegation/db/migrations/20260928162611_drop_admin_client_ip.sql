-- Modify "admin_operation_audit_logs" table
ALTER TABLE "delegation"."admin_operation_audit_logs" DROP COLUMN "client_ip";
-- Modify "delegated_operation_sessions" table
ALTER TABLE "delegation"."delegated_operation_sessions" DROP COLUMN "create_client_ip";
