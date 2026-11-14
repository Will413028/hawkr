-- Add new schema named "adminaudit"
CREATE SCHEMA "adminaudit";
-- Create "records" table
CREATE TABLE "adminaudit"."records" (
  "id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "occurred_at" timestamptz NOT NULL,
  "admin_id" bigint NOT NULL,
  "action" text NOT NULL,
  "target_type" text NOT NULL,
  "target_id" text NOT NULL,
  "detail" jsonb NOT NULL DEFAULT '{}',
  PRIMARY KEY ("id"),
  CONSTRAINT "records_action_check" CHECK ((char_length(action) >= 1) AND (char_length(action) <= 200)),
  CONSTRAINT "records_admin_id_check" CHECK (admin_id > 0),
  CONSTRAINT "records_detail_check" CHECK ((jsonb_typeof(detail) = 'object'::text) AND (octet_length((detail)::text) <= 2048)),
  CONSTRAINT "records_target_id_check" CHECK ((char_length(target_id) >= 1) AND (char_length(target_id) <= 255)),
  CONSTRAINT "records_target_type_check" CHECK (target_type ~ '^[a-z][a-z0-9_]{0,39}$'::text)
);
-- Create index "records_by_action" to table: "records"
CREATE INDEX "records_by_action" ON "adminaudit"."records" ("action", "occurred_at");
-- Create index "records_by_admin" to table: "records"
CREATE INDEX "records_by_admin" ON "adminaudit"."records" ("admin_id", "occurred_at");
-- Create index "records_by_target" to table: "records"
CREATE INDEX "records_by_target" ON "adminaudit"."records" ("target_type", "target_id", "occurred_at");
-- Create index "records_by_time" to table: "records"
CREATE INDEX "records_by_time" ON "adminaudit"."records" ("occurred_at");
