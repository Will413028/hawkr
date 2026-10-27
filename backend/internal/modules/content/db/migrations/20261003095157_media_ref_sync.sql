-- Modify "banners" table
ALTER TABLE "content"."banners" ADD COLUMN "preview_image_asset_id" text NOT NULL, ADD COLUMN "popup_image_asset_id" text NOT NULL;
-- Modify "recipes" table
ALTER TABLE "content"."recipes" ADD COLUMN "image_asset_id" text NOT NULL;
-- Create "media_ref_sync" table
CREATE TABLE "content"."media_ref_sync" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "seq" bigint NOT NULL DEFAULT 1,
  "synced_seq" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("ref_type", "ref_id")
);
-- Create index "media_ref_sync_unsynced" to table: "media_ref_sync"
CREATE INDEX "media_ref_sync_unsynced" ON "content"."media_ref_sync" ("updated_at") WHERE (synced_seq < seq);
-- Create "retired_assets" table
CREATE TABLE "content"."retired_assets" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "asset_id" text NOT NULL,
  "retired_at" timestamptz NOT NULL,
  "unlinked_at" timestamptz NULL,
  PRIMARY KEY ("ref_type", "ref_id", "asset_id")
);
-- Create index "retired_assets_retired_at" to table: "retired_assets"
CREATE INDEX "retired_assets_retired_at" ON "content"."retired_assets" ("retired_at");
