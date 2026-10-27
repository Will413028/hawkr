-- Modify "device_tokens" table
ALTER TABLE "device_tokens" ALTER COLUMN "id" DROP DEFAULT;
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "device_tokens_id_seq";
-- Drop index "idx_notifications_cleanup_failed" from table: "notifications"
DROP INDEX "idx_notifications_cleanup_failed";
-- Rename a constraint from "push_notifications_pkey" to "notifications_pkey"
ALTER TABLE "notifications" RENAME CONSTRAINT "push_notifications_pkey" TO "notifications_pkey";
-- Create index "idx_notifications_cleanup_failed" to table: "notifications"
CREATE INDEX "idx_notifications_cleanup_failed" ON "notifications" ("created_at") WHERE ((status)::text = ANY ((ARRAY['failed'::character varying, 'skipped'::character varying])::text[]));
