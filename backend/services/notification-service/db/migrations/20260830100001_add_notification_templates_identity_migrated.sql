-- Modify "notification_templates" table
ALTER TABLE "notification_templates" ADD COLUMN "identity_migrated" boolean NOT NULL DEFAULT false;
