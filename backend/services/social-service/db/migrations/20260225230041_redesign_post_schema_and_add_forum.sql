-- Drop index "idx_cms_comment_parent" from table: "cms_comment"
DROP INDEX "idx_cms_comment_parent";
-- Drop index "idx_cms_comment_post" from table: "cms_comment"
DROP INDEX "idx_cms_comment_post";
-- Modify "cms_comment" table
ALTER TABLE "cms_comment" DROP COLUMN "status", DROP COLUMN "deleted", ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "idx_cms_comment_parent" to table: "cms_comment"
CREATE INDEX "idx_cms_comment_parent" ON "cms_comment" ("parent_id", "deleted_at", "create_time" DESC);
-- Create index "idx_cms_comment_post" to table: "cms_comment"
CREATE INDEX "idx_cms_comment_post" ON "cms_comment" ("post_id", "parent_id", "deleted_at", "create_time");
-- Drop index "idx_cms_post_seller" from table: "cms_post"
DROP INDEX "idx_cms_post_seller";
-- Modify "cms_post" table
ALTER TABLE "cms_post" DROP COLUMN "title", DROP COLUMN "content_type", DROP COLUMN "images", DROP COLUMN "products", DROP COLUMN "status", DROP COLUMN "deleted", ADD COLUMN "image" text NULL, ADD COLUMN "product_id" bigint NULL, ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "idx_cms_post_seller" to table: "cms_post"
CREATE INDEX "idx_cms_post_seller" ON "cms_post" ("seller_id", "deleted_at", "create_time" DESC);
-- Create "cms_seller_forum" table
CREATE TABLE "cms_seller_forum" (
  "id" bigserial NOT NULL,
  "seller_id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "user_name" character varying(100) NULL,
  "user_avatar" character varying(500) NULL,
  "content" text NOT NULL,
  "parent_id" bigint NOT NULL DEFAULT 0,
  "reply_to_id" bigint NOT NULL DEFAULT 0,
  "reply_to_name" character varying(100) NULL,
  "reply_count" integer NOT NULL DEFAULT 0,
  "deleted_at" timestamptz NULL,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  "update_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_cms_seller_forum_parent" to table: "cms_seller_forum"
CREATE INDEX "idx_cms_seller_forum_parent" ON "cms_seller_forum" ("parent_id", "deleted_at", "create_time" DESC);
-- Create index "idx_cms_seller_forum_seller" to table: "cms_seller_forum"
CREATE INDEX "idx_cms_seller_forum_seller" ON "cms_seller_forum" ("seller_id", "parent_id", "deleted_at", "create_time" DESC);
