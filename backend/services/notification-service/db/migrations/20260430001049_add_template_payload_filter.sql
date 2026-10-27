-- Modify "notification_templates" table
ALTER TABLE "notification_templates" ADD COLUMN "payload_filter" jsonb NULL;
