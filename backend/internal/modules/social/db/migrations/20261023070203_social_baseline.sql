-- Add new schema named "social"
CREATE SCHEMA "social";
-- Create "inactive_authors" table
CREATE TABLE "social"."inactive_authors" (
  "buyer_id" bigint NOT NULL,
  "since" timestamptz NOT NULL,
  PRIMARY KEY ("buyer_id"),
  CONSTRAINT "inactive_authors_buyer_check" CHECK (buyer_id > 0)
);
-- Create "media_ref_sync" table
CREATE TABLE "social"."media_ref_sync" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "seq" bigint NOT NULL DEFAULT 1,
  "synced_seq" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("ref_type", "ref_id")
);
-- Create index "media_ref_sync_unsynced" to table: "media_ref_sync"
CREATE INDEX "media_ref_sync_unsynced" ON "social"."media_ref_sync" ("updated_at") WHERE (synced_seq < seq);
-- Create "posts" table
CREATE TABLE "social"."posts" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "body" text NOT NULL DEFAULT '',
  "image_key" text NULL,
  "image_asset_id" text NULL,
  "product_id" bigint NULL,
  "status" text NOT NULL,
  "published_at" timestamptz NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "posts_body_check" CHECK ((char_length(body) <= 2200) AND (body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "posts_image_check" CHECK ((image_key IS NULL) = (image_asset_id IS NULL)),
  CONSTRAINT "posts_product_check" CHECK (product_id > 0),
  CONSTRAINT "posts_published_at_check" CHECK ((status = 'DRAFT'::text) = (published_at IS NULL)),
  CONSTRAINT "posts_status_check" CHECK (status = ANY (ARRAY['DRAFT'::text, 'PUBLISHED'::text, 'UNPUBLISHED'::text])),
  CONSTRAINT "posts_store_check" CHECK (store_id > 0),
  CONSTRAINT "posts_version_check" CHECK (version >= 1)
);
-- Create index "posts_feed" to table: "posts"
CREATE INDEX "posts_feed" ON "social"."posts" ("published_at" DESC, "id" DESC) WHERE ((status = 'PUBLISHED'::text) AND (deleted_at IS NULL));
-- Create index "posts_store_feed" to table: "posts"
CREATE INDEX "posts_store_feed" ON "social"."posts" ("store_id", "published_at" DESC, "id" DESC) WHERE ((status = 'PUBLISHED'::text) AND (deleted_at IS NULL));
-- Create index "posts_store_mine" to table: "posts"
CREATE INDEX "posts_store_mine" ON "social"."posts" ("store_id", "created_at" DESC, "id" DESC) WHERE (deleted_at IS NULL);
-- Set comment to column: "status" on table: "posts"
COMMENT ON COLUMN "social"."posts"."status" IS '貼文狀態：DRAFT、PUBLISHED、UNPUBLISHED（儲存的代碼是 UPPER_SNAKE，舊的 publish_status 4401、4402、4403 依序對應）';
-- Set comment to column: "published_at" on table: "posts"
COMMENT ON COLUMN "social"."posts"."published_at" IS '第一次發布的時間，之後不變（下架不清、再發布沿用）；DRAFT 沒有，發布過的一定有';
-- Set comment to column: "version" on table: "posts"
COMMENT ON COLUMN "social"."posts"."version" IS '樂觀鎖的版本，對外是 etag；每次修改加一';
-- Create "retired_assets" table
CREATE TABLE "social"."retired_assets" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "asset_id" text NOT NULL,
  "retired_at" timestamptz NOT NULL,
  "unlinked_at" timestamptz NULL,
  PRIMARY KEY ("ref_type", "ref_id", "asset_id")
);
-- Create index "retired_assets_retired_at" to table: "retired_assets"
CREATE INDEX "retired_assets_retired_at" ON "social"."retired_assets" ("retired_at");
-- Create "sanctions" table
CREATE TABLE "social"."sanctions" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "kind" text NOT NULL,
  "reason_category" text NOT NULL,
  "reason_text" text NOT NULL DEFAULT '',
  "source_ref" text NULL,
  "admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  "ends_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sanctions_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "sanctions_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "sanctions_ends_at_check" CHECK ((ends_at IS NULL) OR (ends_at > created_at)),
  CONSTRAINT "sanctions_kind_check" CHECK (kind = 'WARNING'::text),
  CONSTRAINT "sanctions_reason_category_check" CHECK (reason_category = ANY (ARRAY['MISLEADING_INFO'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text, 'SPAM'::text, 'HARASSMENT'::text, 'HATE'::text])),
  CONSTRAINT "sanctions_reason_text_check" CHECK ((char_length(reason_text) <= 500) AND (reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "sanctions_source_ref_check" CHECK ((source_ref IS NULL) OR ((char_length(source_ref) >= 1) AND (char_length(source_ref) <= 200) AND (source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)))
);
-- Create index "sanctions_buyer" to table: "sanctions"
CREATE INDEX "sanctions_buyer" ON "social"."sanctions" ("buyer_id", "created_at" DESC, "id" DESC);
-- Create "view_salts" table
CREATE TABLE "social"."view_salts" (
  "view_day" date NOT NULL,
  "salt" bytea NOT NULL,
  PRIMARY KEY ("view_day"),
  CONSTRAINT "view_salts_salt_check" CHECK (octet_length(salt) = 32)
);
-- Create "comments" table
CREATE TABLE "social"."comments" (
  "id" uuid NOT NULL,
  "post_id" uuid NOT NULL,
  "parent_id" uuid NULL,
  "reply_to_id" uuid NULL,
  "actor_kind" text NOT NULL,
  "actor_id" bigint NOT NULL,
  "body" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  "thread_id" uuid NOT NULL GENERATED ALWAYS AS (COALESCE(parent_id, id)) STORED,
  "counted" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "comments_post_id_key" UNIQUE ("post_id", "id"),
  CONSTRAINT "comments_thread_key" UNIQUE ("post_id", "thread_id", "id"),
  CONSTRAINT "comments_parent_fk" FOREIGN KEY ("post_id", "parent_id") REFERENCES "social"."comments" ("post_id", "id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "comments_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "comments_reply_to_fk" FOREIGN KEY ("post_id", "parent_id", "reply_to_id") REFERENCES "social"."comments" ("post_id", "thread_id", "id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "comments_actor_id_check" CHECK (actor_id > 0),
  CONSTRAINT "comments_actor_kind_check" CHECK (actor_kind = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "comments_body_check" CHECK ((char_length(body) >= 1) AND (char_length(body) <= 2200) AND (body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "comments_parent_check" CHECK ((parent_id IS NULL) OR (parent_id <> id)),
  CONSTRAINT "comments_reply_to_check" CHECK ((reply_to_id IS NULL) OR ((parent_id IS NOT NULL) AND (reply_to_id <> id))),
  CONSTRAINT "comments_scrubbed_check" CHECK ((body IS NOT NULL) OR (deleted_at IS NOT NULL))
);
-- Create index "comments_actor" to table: "comments"
CREATE INDEX "comments_actor" ON "social"."comments" ("actor_kind", "actor_id", "id");
-- Create index "comments_replies" to table: "comments"
CREATE INDEX "comments_replies" ON "social"."comments" ("parent_id", "created_at", "id") WHERE counted;
-- Create index "comments_top_level" to table: "comments"
CREATE INDEX "comments_top_level" ON "social"."comments" ("post_id", "created_at", "id") WHERE (parent_id IS NULL);
-- Set comment to column: "actor_kind" on table: "comments"
COMMENT ON COLUMN "social"."comments"."actor_kind" IS '互動者的種類：BUYER（actor_id 是 buyer_id）、STORE（actor_id 是 store_id）；買家與店家各自連號，同一個數值在兩邊是不同的人';
-- Set comment to column: "body" on table: "comments"
COMMENT ON COLUMN "social"."comments"."body" IS '內文；作者自己刪除時保留，買家刪帳的清除 job 之後設為 NULL（保留列讓討論串結構與佔位項目還在），所以 NULL 一定是已刪除的列（comments_scrubbed_check）';
-- Set comment to column: "counted" on table: "comments"
COMMENT ON COLUMN "social"."comments"."counted" IS '此刻是否計入貼文的留言數與父留言的回覆數（沒有刪除、沒有未還原的隱藏、作者不在 inactive_authors，在 social.reevaluate_comments 求值）；只由 social.reevaluate_comments 維護，讀取直接用它，貼文的 comment_count 與留言的 reply_count 是這個旗標的加總';
-- Create "comment_likes" table
CREATE TABLE "social"."comment_likes" (
  "comment_id" uuid NOT NULL,
  "actor_kind" text NOT NULL,
  "actor_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("comment_id", "actor_kind", "actor_id"),
  CONSTRAINT "comment_likes_comment_id_fkey" FOREIGN KEY ("comment_id") REFERENCES "social"."comments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "comment_likes_actor_id_check" CHECK (actor_id > 0),
  CONSTRAINT "comment_likes_actor_kind_check" CHECK (actor_kind = ANY (ARRAY['BUYER'::text, 'STORE'::text]))
);
-- Create index "comment_likes_actor" to table: "comment_likes"
CREATE INDEX "comment_likes_actor" ON "social"."comment_likes" ("actor_kind", "actor_id", "comment_id");
-- Create "comment_stats" table
CREATE TABLE "social"."comment_stats" (
  "comment_id" uuid NOT NULL,
  "like_count" bigint NOT NULL DEFAULT 0,
  "reply_count" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("comment_id"),
  CONSTRAINT "comment_stats_comment_id_fkey" FOREIGN KEY ("comment_id") REFERENCES "social"."comments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "comment_stats_counts_check" CHECK ((like_count >= 0) AND (reply_count >= 0))
);
-- Create "forum_entries" table
CREATE TABLE "social"."forum_entries" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "parent_id" uuid NULL,
  "reply_to_id" uuid NULL,
  "actor_kind" text NOT NULL,
  "actor_id" bigint NOT NULL,
  "body" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "deleted_at" timestamptz NULL,
  "thread_id" uuid NOT NULL GENERATED ALWAYS AS (COALESCE(parent_id, id)) STORED,
  "counted" boolean NOT NULL DEFAULT false,
  PRIMARY KEY ("id"),
  CONSTRAINT "forum_entries_store_id_key" UNIQUE ("store_id", "id"),
  CONSTRAINT "forum_entries_thread_key" UNIQUE ("store_id", "thread_id", "id"),
  CONSTRAINT "forum_entries_parent_fk" FOREIGN KEY ("store_id", "parent_id") REFERENCES "social"."forum_entries" ("store_id", "id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "forum_entries_reply_to_fk" FOREIGN KEY ("store_id", "parent_id", "reply_to_id") REFERENCES "social"."forum_entries" ("store_id", "thread_id", "id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "forum_entries_actor_id_check" CHECK (actor_id > 0),
  CONSTRAINT "forum_entries_actor_kind_check" CHECK (actor_kind = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "forum_entries_body_check" CHECK ((char_length(body) >= 1) AND (char_length(body) <= 2200) AND (body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "forum_entries_parent_check" CHECK ((parent_id IS NULL) OR (parent_id <> id)),
  CONSTRAINT "forum_entries_reply_to_check" CHECK ((reply_to_id IS NULL) OR ((parent_id IS NOT NULL) AND (reply_to_id <> id))),
  CONSTRAINT "forum_entries_scrubbed_check" CHECK ((body IS NOT NULL) OR (deleted_at IS NOT NULL)),
  CONSTRAINT "forum_entries_store_check" CHECK (store_id > 0)
);
-- Create index "forum_entries_actor" to table: "forum_entries"
CREATE INDEX "forum_entries_actor" ON "social"."forum_entries" ("actor_kind", "actor_id", "id");
-- Create index "forum_entries_replies" to table: "forum_entries"
CREATE INDEX "forum_entries_replies" ON "social"."forum_entries" ("parent_id", "created_at", "id") WHERE counted;
-- Create index "forum_entries_top_level" to table: "forum_entries"
CREATE INDEX "forum_entries_top_level" ON "social"."forum_entries" ("store_id", "created_at", "id") WHERE (parent_id IS NULL);
-- Set comment to column: "body" on table: "forum_entries"
COMMENT ON COLUMN "social"."forum_entries"."body" IS '內文；作者自己刪除時保留，買家刪帳的清除 job 之後設為 NULL（保留列讓討論串結構與佔位項目還在），所以 NULL 一定是已刪除的列（forum_entries_scrubbed_check）';
-- Set comment to column: "counted" on table: "forum_entries"
COMMENT ON COLUMN "social"."forum_entries"."counted" IS '此刻是否計入父討論區留言的回覆數（沒有刪除、沒有未還原的隱藏、作者不在 inactive_authors，在 social.reevaluate_forum_entries 求值）；只由 social.reevaluate_forum_entries 維護，讀取直接用它，父項的 reply_count 是這個旗標的加總';
-- Create "forum_entry_stats" table
CREATE TABLE "social"."forum_entry_stats" (
  "forum_entry_id" uuid NOT NULL,
  "reply_count" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("forum_entry_id"),
  CONSTRAINT "forum_entry_stats_forum_entry_id_fkey" FOREIGN KEY ("forum_entry_id") REFERENCES "social"."forum_entries" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "forum_entry_stats_counts_check" CHECK (reply_count >= 0)
);
-- Create "hides" table
CREATE TABLE "social"."hides" (
  "id" uuid NOT NULL,
  "comment_id" uuid NULL,
  "forum_entry_id" uuid NULL,
  "reason_category" text NOT NULL,
  "reason_text" text NOT NULL DEFAULT '',
  "source_ref" text NULL,
  "admin_id" bigint NOT NULL,
  "hidden_at" timestamptz NOT NULL,
  "restored_at" timestamptz NULL,
  "restored_by" bigint NULL,
  "restore_reason" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "hides_comment_id_fkey" FOREIGN KEY ("comment_id") REFERENCES "social"."comments" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "hides_forum_entry_id_fkey" FOREIGN KEY ("forum_entry_id") REFERENCES "social"."forum_entries" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "hides_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "hides_reason_category_check" CHECK (reason_category = ANY (ARRAY['MISLEADING_INFO'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text, 'SPAM'::text, 'HARASSMENT'::text, 'HATE'::text])),
  CONSTRAINT "hides_reason_text_check" CHECK ((char_length(reason_text) <= 500) AND (reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "hides_restore_check" CHECK (((restored_at IS NULL) AND (restored_by IS NULL) AND (restore_reason IS NULL)) OR ((restored_at IS NOT NULL) AND (restored_by > 0) AND (restore_reason IS NOT NULL) AND (restored_at >= hidden_at))),
  CONSTRAINT "hides_restore_reason_check" CHECK ((restore_reason IS NULL) OR ((char_length(restore_reason) >= 1) AND (char_length(restore_reason) <= 500) AND (restore_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "hides_source_ref_check" CHECK ((source_ref IS NULL) OR ((char_length(source_ref) >= 1) AND (char_length(source_ref) <= 200) AND (source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "hides_target_check" CHECK (num_nonnulls(comment_id, forum_entry_id) = 1)
);
-- Create index "hides_active_comment" to table: "hides"
CREATE UNIQUE INDEX "hides_active_comment" ON "social"."hides" ("comment_id") WHERE ((restored_at IS NULL) AND (comment_id IS NOT NULL));
-- Create index "hides_active_forum_entry" to table: "hides"
CREATE UNIQUE INDEX "hides_active_forum_entry" ON "social"."hides" ("forum_entry_id") WHERE ((restored_at IS NULL) AND (forum_entry_id IS NOT NULL));
-- Create index "hides_comment" to table: "hides"
CREATE INDEX "hides_comment" ON "social"."hides" ("comment_id", "hidden_at") WHERE (comment_id IS NOT NULL);
-- Create index "hides_forum_entry" to table: "hides"
CREATE INDEX "hides_forum_entry" ON "social"."hides" ("forum_entry_id", "hidden_at") WHERE (forum_entry_id IS NOT NULL);
-- Create "post_hashtags" table
CREATE TABLE "social"."post_hashtags" (
  "post_id" uuid NOT NULL,
  "position" smallint NOT NULL,
  "name" text NOT NULL,
  "display_name" text NOT NULL,
  "published_at" timestamptz NULL,
  PRIMARY KEY ("post_id", "position"),
  CONSTRAINT "post_hashtags_name_key" UNIQUE ("post_id", "name"),
  CONSTRAINT "post_hashtags_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "post_hashtags_display_check" CHECK ((char_length(display_name) >= 1) AND (char_length(display_name) <= 40)),
  CONSTRAINT "post_hashtags_name_check" CHECK ((char_length(name) >= 1) AND (char_length(name) <= 40)),
  CONSTRAINT "post_hashtags_position_check" CHECK (("position" >= 0) AND ("position" <= 9))
);
-- Create index "post_hashtags_feed" to table: "post_hashtags"
CREATE INDEX "post_hashtags_feed" ON "social"."post_hashtags" ("name", "published_at" DESC, "post_id" DESC) WHERE (published_at IS NOT NULL);
-- Create index "post_hashtags_name" to table: "post_hashtags"
CREATE INDEX "post_hashtags_name" ON "social"."post_hashtags" ("name", "post_id");
-- Set comment to column: "published_at" on table: "post_hashtags"
COMMENT ON COLUMN "social"."post_hashtags"."published_at" IS '貼文 posts.published_at 的複本（trigger 維護，貼文發布後不變）；草稿的標籤列是 NULL';
-- Create "post_likes" table
CREATE TABLE "social"."post_likes" (
  "post_id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("post_id", "buyer_id"),
  CONSTRAINT "post_likes_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "post_likes_buyer_check" CHECK (buyer_id > 0)
);
-- Create index "post_likes_buyer" to table: "post_likes"
CREATE INDEX "post_likes_buyer" ON "social"."post_likes" ("buyer_id", "post_id");
-- Create "post_shares" table
CREATE TABLE "social"."post_shares" (
  "post_id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("post_id", "buyer_id"),
  CONSTRAINT "post_shares_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "post_shares_buyer_check" CHECK (buyer_id > 0)
);
-- Create index "post_shares_buyer" to table: "post_shares"
CREATE INDEX "post_shares_buyer" ON "social"."post_shares" ("buyer_id", "post_id");
-- Create "post_stats" table
CREATE TABLE "social"."post_stats" (
  "post_id" uuid NOT NULL,
  "like_count" bigint NOT NULL DEFAULT 0,
  "comment_count" bigint NOT NULL DEFAULT 0,
  "view_count" bigint NOT NULL DEFAULT 0,
  "share_count" bigint NOT NULL DEFAULT 0,
  PRIMARY KEY ("post_id"),
  CONSTRAINT "post_stats_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "post_stats_counts_check" CHECK ((like_count >= 0) AND (comment_count >= 0) AND (view_count >= 0) AND (share_count >= 0))
);
-- Set comment to column: "comment_count" on table: "post_stats"
COMMENT ON COLUMN "social"."post_stats"."comment_count" IS '留言數含回覆：旗標 comments.counted 為真的列數；計入的定義在 social.reevaluate_comments（只讀 social 自己的表）';
-- Set comment to column: "view_count" on table: "post_stats"
COMMENT ON COLUMN "social"."post_stats"."view_count" IS '只增的事件計數（去重後的瀏覽），沒有可重算的來源，不進每日對帳';
-- Create "post_views" table
CREATE TABLE "social"."post_views" (
  "post_id" uuid NOT NULL,
  "view_day" date NOT NULL,
  "viewer_key" bytea NOT NULL,
  PRIMARY KEY ("post_id", "view_day", "viewer_key"),
  CONSTRAINT "post_views_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "post_views_viewer_key_check" CHECK (octet_length(viewer_key) = 32)
);
-- Create index "post_views_day" to table: "post_views"
CREATE INDEX "post_views_day" ON "social"."post_views" ("view_day");
