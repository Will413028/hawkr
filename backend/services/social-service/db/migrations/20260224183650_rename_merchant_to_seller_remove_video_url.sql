-- Create "cms_comment" table
CREATE TABLE "cms_comment" (
  "id" bigserial NOT NULL,
  "post_id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "user_name" character varying(100) NULL,
  "user_avatar" character varying(500) NULL,
  "content" text NULL,
  "parent_id" bigint NOT NULL DEFAULT 0,
  "reply_to_id" bigint NOT NULL DEFAULT 0,
  "reply_to_name" character varying(100) NULL,
  "like_count" integer NOT NULL DEFAULT 0,
  "reply_count" integer NOT NULL DEFAULT 0,
  "status" integer NOT NULL DEFAULT 1,
  "deleted" integer NOT NULL DEFAULT 0,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  "update_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_cms_comment_parent" to table: "cms_comment"
CREATE INDEX "idx_cms_comment_parent" ON "cms_comment" ("parent_id", "deleted", "create_time" DESC);
-- Create index "idx_cms_comment_post" to table: "cms_comment"
CREATE INDEX "idx_cms_comment_post" ON "cms_comment" ("post_id", "parent_id", "deleted", "create_time");
-- Create "cms_comment_like" table
CREATE TABLE "cms_comment_like" (
  "id" bigserial NOT NULL,
  "comment_id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "cms_comment_like_comment_id_user_id_key" UNIQUE ("comment_id", "user_id")
);
-- Create "cms_post" table
CREATE TABLE "cms_post" (
  "id" bigserial NOT NULL,
  "seller_id" bigint NOT NULL,
  "title" character varying(255) NULL,
  "content" text NULL,
  "content_type" character varying(50) NULL,
  "images" text NULL,
  "products" text NULL,
  "like_count" integer NOT NULL DEFAULT 0,
  "comment_count" integer NOT NULL DEFAULT 0,
  "view_count" integer NOT NULL DEFAULT 0,
  "share_count" integer NOT NULL DEFAULT 0,
  "status" integer NOT NULL DEFAULT 1,
  "deleted" integer NOT NULL DEFAULT 0,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  "update_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_cms_post_seller" to table: "cms_post"
CREATE INDEX "idx_cms_post_seller" ON "cms_post" ("seller_id", "deleted", "create_time" DESC);
-- Create index "idx_cms_post_status" to table: "cms_post"
CREATE INDEX "idx_cms_post_status" ON "cms_post" ("status", "deleted", "create_time" DESC);
-- Create "cms_post_like" table
CREATE TABLE "cms_post_like" (
  "id" bigserial NOT NULL,
  "post_id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "cms_post_like_post_id_user_id_key" UNIQUE ("post_id", "user_id")
);
-- Create "cms_post_save" table
CREATE TABLE "cms_post_save" (
  "id" bigserial NOT NULL,
  "post_id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "cms_post_save_post_id_user_id_key" UNIQUE ("post_id", "user_id")
);
