-- lock_timeout：GENERATED ... STORED 走不了 fast-default 路徑，這支 ALTER 會用 ACCESS
-- EXCLUSIVE 重寫整張表（連讀都擋），後面的 CREATE INDEX（非 CONCURRENTLY）再疊一次
-- SHARE。repo 的 migrate 路徑目前沒有任何一處設 lock_timeout：若這兩句排在一個長查詢
-- 後面，會卡住 cms_post 上之後所有的讀寫，不只是寫——整個貼文功能停擺，而不是等待。
-- 3s 之後寧可讓這次 migration 失敗改期，不要讓它啞掉整張表。
SET lock_timeout = '3s';
-- Modify "cms_post" table
ALTER TABLE "cms_post" ADD COLUMN "image_asset_id" bigint NULL, ADD COLUMN "media_converged_asset_id" bigint NULL, ADD COLUMN "media_next_attempt_at" timestamptz NULL, ADD COLUMN "media_desired_asset_id" bigint NULL GENERATED ALWAYS AS (
CASE
    WHEN (deleted_at IS NOT NULL) THEN NULL::bigint
    ELSE image_asset_id
END) STORED;
-- Create index "idx_cms_post_media_diverged" to table: "cms_post"
CREATE INDEX "idx_cms_post_media_diverged" ON "cms_post" ("update_time", "id") WHERE (media_desired_asset_id IS DISTINCT FROM media_converged_asset_id);
