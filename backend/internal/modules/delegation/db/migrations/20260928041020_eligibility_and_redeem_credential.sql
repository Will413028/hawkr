-- Modify "delegated_operation_sessions" table
ALTER TABLE "delegation"."delegated_operation_sessions" ALTER COLUMN "credential_hash" DROP NOT NULL;
-- Create "admin_eligibilities" table
CREATE TABLE "delegation"."admin_eligibilities" (
  "admin_id" bigint NOT NULL,
  PRIMARY KEY ("admin_id")
);
