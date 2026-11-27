-- Create "admin_operation_audit_log_items" table
CREATE TABLE "delegation"."admin_operation_audit_log_items" (
  "audit_log_id" uuid NOT NULL,
  "resource_id" text NOT NULL,
  "resource_name" text NULL,
  "resource_version_before" bigint NULL,
  "resource_version_after" bigint NULL,
  PRIMARY KEY ("audit_log_id", "resource_id"),
  CONSTRAINT "admin_operation_audit_log_items_audit_log_id_fkey" FOREIGN KEY ("audit_log_id") REFERENCES "delegation"."admin_operation_audit_logs" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
