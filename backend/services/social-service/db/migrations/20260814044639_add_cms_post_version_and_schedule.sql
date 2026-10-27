-- Modify "cms_post" table
ALTER TABLE "cms_post" ADD CONSTRAINT "chk_cms_post_schedule_pending_at" CHECK ((schedule_state IS DISTINCT FROM 'pending'::text) OR (scheduled_publish_at IS NOT NULL)), ADD CONSTRAINT "chk_cms_post_schedule_state" CHECK ((schedule_state IS NULL) OR (schedule_state = ANY (ARRAY['pending'::text, 'done'::text, 'expired'::text, 'cancelled_by_revocation'::text, 'cancelled_by_deletion'::text, 'cancelled_by_governance'::text]))), ADD CONSTRAINT "chk_cms_post_version_nonneg" CHECK (version >= 0), ADD COLUMN "version" bigint NOT NULL DEFAULT 0, ADD COLUMN "publish_status" smallint NOT NULL DEFAULT 4402, ADD COLUMN "scheduled_publish_at" timestamptz NULL, ADD COLUMN "published_at" timestamptz NULL, ADD COLUMN "schedule_state" text NULL, ADD COLUMN "scheduled_by_session_id" bigint NULL, ADD COLUMN "schedule_lease_owner" text NULL, ADD COLUMN "schedule_lease_expires_at" timestamptz NULL, ADD COLUMN "schedule_next_attempt_at" timestamptz NULL, ADD COLUMN "schedule_retry_count" integer NOT NULL DEFAULT 0;
-- Create index "idx_cms_post_schedule_pending" to table: "cms_post"
CREATE INDEX "idx_cms_post_schedule_pending" ON "cms_post" ("scheduled_publish_at") WHERE (schedule_state = 'pending'::text);
-- 回填 published_at：既有貼文的發布時點就是它的 create_time。publish_status 靠 DEFAULT 4402
-- 達成回填，published_at 沒有可依賴的 DEFAULT，必須顯式補（漏了在功能上完全無感——草稿過濾
-- 與排程都不讀這一欄，要等有人問「這篇什麼時候發的」才會發現）。
UPDATE "cms_post" SET "published_at" = "create_time" WHERE "published_at" IS NULL;
