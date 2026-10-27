-- Modify "notification_templates" table
ALTER TABLE "notification_templates" ADD COLUMN "image_url_template" text NULL;
-- Create "system_notification_logs" table
CREATE TABLE "system_notification_logs" (
  "id" bigint NOT NULL,
  "notification_type" character varying(40) NOT NULL,
  "title" text NOT NULL,
  "body" text NOT NULL,
  "image_url" text NULL,
  "deep_link_url" text NULL,
  "content_hash" character(64) NOT NULL,
  "triggered_by_account_id" bigint NOT NULL,
  "triggered_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "fcm_message_id" text NULL,
  "status" character varying(20) NOT NULL DEFAULT 'pending',
  "failed_reason" text NULL,
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_system_notification_logs_content_hash" to table: "system_notification_logs"
CREATE INDEX "idx_system_notification_logs_content_hash" ON "system_notification_logs" ("content_hash", "triggered_at" DESC);
-- Create index "idx_system_notification_logs_triggered_at" to table: "system_notification_logs"
CREATE INDEX "idx_system_notification_logs_triggered_at" ON "system_notification_logs" ("triggered_at" DESC);
