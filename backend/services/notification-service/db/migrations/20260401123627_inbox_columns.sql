-- Modify "push_notifications" table
ALTER TABLE "push_notifications" ADD COLUMN "is_read" boolean NOT NULL DEFAULT false, ADD COLUMN "read_at" timestamptz NULL, ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "idx_push_notifications_inbox" to table: "push_notifications"
CREATE INDEX "idx_push_notifications_inbox" ON "push_notifications" ("user_id", "user_type", "created_at" DESC) WHERE (((status)::text = 'sent'::text) AND (deleted_at IS NULL));
-- Create index "idx_push_notifications_unread" to table: "push_notifications"
CREATE INDEX "idx_push_notifications_unread" ON "push_notifications" ("user_id", "user_type") WHERE (((status)::text = 'sent'::text) AND (deleted_at IS NULL) AND (is_read = false));
