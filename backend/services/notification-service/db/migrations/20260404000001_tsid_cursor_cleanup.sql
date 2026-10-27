-- 重新命名 push_notifications → notifications
ALTER TABLE "push_notifications" RENAME TO "notifications";

-- 重新命名 index
ALTER INDEX "idx_push_notifications_user" RENAME TO "idx_notifications_user";
ALTER INDEX "idx_push_notifications_dedup" RENAME TO "idx_notifications_dedup";
ALTER INDEX "idx_push_notifications_unread" RENAME TO "idx_notifications_unread";

-- notifications: bigserial → bigint (TSID，由應用層產生)
ALTER TABLE "notifications" ALTER COLUMN "id" DROP DEFAULT;
DROP SEQUENCE IF EXISTS push_notifications_id_seq;

-- notification_templates: bigserial → bigint (TSID)
ALTER TABLE "notification_templates" ALTER COLUMN "id" DROP DEFAULT;
DROP SEQUENCE IF EXISTS notification_templates_id_seq;

-- inbox index 改用 id DESC（配合 TSID cursor-based pagination）
DROP INDEX IF EXISTS "idx_push_notifications_inbox";
CREATE INDEX "idx_notifications_inbox" ON "notifications" ("user_id", "user_type", "id" DESC) WHERE ((("status")::text = 'sent'::text) AND ("deleted_at" IS NULL));

-- cleanup job 用的 partial index
CREATE INDEX "idx_notifications_cleanup_failed" ON "notifications" ("created_at") WHERE ((("status")::text = ANY (ARRAY[('failed'::character varying)::text, ('skipped'::character varying)::text])));
CREATE INDEX "idx_notifications_cleanup_deleted" ON "notifications" ("deleted_at") WHERE ("deleted_at" IS NOT NULL);
