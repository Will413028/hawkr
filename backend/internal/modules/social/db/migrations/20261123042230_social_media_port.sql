-- Existing post image asset IDs are Java media TSIDs and cannot be read as UUIDs; the system has not launched, so clear the old post images.
UPDATE "social"."posts" SET "image_key" = NULL, "image_asset_id" = NULL WHERE "image_asset_id" IS NOT NULL;
-- Modify "posts" table
ALTER TABLE "social"."posts" ALTER COLUMN "image_asset_id" TYPE uuid USING "image_asset_id"::uuid;
-- Drop "media_ref_sync" table
DROP TABLE "social"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "social"."retired_assets";
