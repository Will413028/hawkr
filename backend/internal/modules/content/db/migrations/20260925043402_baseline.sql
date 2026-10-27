-- Add new schema named "content"
CREATE SCHEMA "content";
-- Create "app_version_gates" table
CREATE TABLE "content"."app_version_gates" (
  "app_type" character varying(16) NOT NULL,
  "platform" character varying(16) NOT NULL,
  "minimum_version" character varying(32) NOT NULL,
  "store_url" text NULL,
  "notice_title" character varying(255) NOT NULL DEFAULT '',
  "notice_body" text NOT NULL DEFAULT '',
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_type", "platform"),
  CONSTRAINT "app_version_gates_app_type_check" CHECK ((app_type)::text = ANY (ARRAY['BUYER'::text, 'SELLER'::text])),
  CONSTRAINT "app_version_gates_minimum_version_check" CHECK ((minimum_version)::text ~ '^[0-9]+\.[0-9]+\.[0-9]+$'::text),
  CONSTRAINT "app_version_gates_platform_check" CHECK ((platform)::text = ANY (ARRAY['IOS'::text, 'ANDROID'::text]))
);
-- Set comment to column: "app_type" on table: "app_version_gates"
COMMENT ON COLUMN "content"."app_version_gates"."app_type" IS 'App 類型 - AppType（BUYER、SELLER）';
-- Set comment to column: "platform" on table: "app_version_gates"
COMMENT ON COLUMN "content"."app_version_gates"."platform" IS 'App 平台 - AppPlatform（IOS、ANDROID）';
-- Create "banners" table
CREATE TABLE "content"."banners" (
  "id" uuid NOT NULL,
  "title" character varying(255) NOT NULL,
  "button_text" character varying(100) NOT NULL DEFAULT '',
  "preview_image_key" text NOT NULL,
  "popup_image_key" text NOT NULL,
  "canvas_json" text NOT NULL DEFAULT '',
  "action" jsonb NOT NULL,
  "display_order" integer NOT NULL DEFAULT 0,
  "is_active" boolean NOT NULL DEFAULT false,
  "start_date" timestamptz NULL,
  "end_date" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "banners_schedule_check" CHECK ((start_date IS NULL) OR (end_date IS NULL) OR (start_date <= end_date))
);
-- Create index "idx_banners_is_active_display_order" to table: "banners"
CREATE INDEX "idx_banners_is_active_display_order" ON "content"."banners" ("is_active", "display_order");
-- Set comment to column: "action" on table: "banners"
COMMENT ON COLUMN "content"."banners"."action" IS '點擊動作：{"type":"popup"}、{"type":"url","url":…}、{"type":"product"|"post","id":…}；由模組驗證，沒有預設值';
-- Create "recipes" table
CREATE TABLE "content"."recipes" (
  "id" uuid NOT NULL,
  "title" character varying(255) NOT NULL,
  "image_key" text NOT NULL,
  "category" character varying(50) NOT NULL DEFAULT '',
  "cooking_time_minutes" integer NULL,
  "servings" integer NULL,
  "view_count" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_recipes_category" to table: "recipes"
CREATE INDEX "idx_recipes_category" ON "content"."recipes" ("category");
-- Create index "idx_recipes_created_at" to table: "recipes"
CREATE INDEX "idx_recipes_created_at" ON "content"."recipes" ("created_at" DESC);
-- Create index "idx_recipes_view_count" to table: "recipes"
CREATE INDEX "idx_recipes_view_count" ON "content"."recipes" ("view_count" DESC);
-- Create "site_contents" table
CREATE TABLE "content"."site_contents" (
  "content_key" character varying(100) NOT NULL,
  "label" character varying(255) NOT NULL DEFAULT '',
  "content_text" text NOT NULL DEFAULT '',
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("content_key")
);
-- Create "recipe_ingredients" table
CREATE TABLE "content"."recipe_ingredients" (
  "recipe_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "name" character varying(255) NOT NULL,
  "amount" character varying(255) NOT NULL DEFAULT '',
  PRIMARY KEY ("recipe_id", "position"),
  CONSTRAINT "recipe_ingredients_recipe_id_fkey" FOREIGN KEY ("recipe_id") REFERENCES "content"."recipes" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "recipe_ingredients_position_check" CHECK ("position" >= 0)
);
-- Create "recipe_steps" table
CREATE TABLE "content"."recipe_steps" (
  "recipe_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "description" text NOT NULL DEFAULT '',
  PRIMARY KEY ("recipe_id", "position"),
  CONSTRAINT "recipe_steps_recipe_id_fkey" FOREIGN KEY ("recipe_id") REFERENCES "content"."recipes" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "recipe_steps_position_check" CHECK ("position" >= 0)
);
