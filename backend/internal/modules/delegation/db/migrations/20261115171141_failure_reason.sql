-- Modify "admin_operation_audit_logs" table
ALTER TABLE "delegation"."admin_operation_audit_logs" ADD COLUMN "failure_reason" text NULL;
-- Backfill: existing FAILED rows have no recorded reason
UPDATE "delegation"."admin_operation_audit_logs" SET "failure_reason" = 'INTERNAL' WHERE "result" = 'FAILED';
ALTER TABLE "delegation"."admin_operation_audit_logs" ADD CONSTRAINT "admin_operation_audit_logs_failure_reason_check" CHECK (((result = 'FAILED'::text) = (failure_reason IS NOT NULL)) AND ((failure_reason IS NULL) OR ((failure_reason ~ '^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$'::text) AND (failure_reason <> 'UNSPECIFIED'::text))));
