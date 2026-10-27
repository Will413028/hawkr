-- Drop index "push_notifications_dedup_idx" from table: "push_notifications"
DROP INDEX "push_notifications_dedup_idx";
-- Create index "push_notifications_dedup_idx" to table: "push_notifications"
CREATE UNIQUE INDEX "push_notifications_dedup_idx" ON "push_notifications" ("user_id", "user_type", "event_type", "reference_id") WHERE (((status)::text = 'sent'::text) AND (reference_id IS NOT NULL));
