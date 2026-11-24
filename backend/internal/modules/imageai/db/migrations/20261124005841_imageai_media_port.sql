-- Existing thumbnail asset IDs are Java media TSIDs and cannot be read as UUIDs; the system has not launched, so remove the non-default styles and clear the default styles' thumbnails (the seeded defaults have none).
ALTER TABLE "imageai"."styles" ALTER COLUMN "thumbnail_asset_id" DROP NOT NULL, ALTER COLUMN "thumbnail_asset_id" DROP DEFAULT;
DELETE FROM "imageai"."styles" WHERE NOT "is_default";
UPDATE "imageai"."styles" SET "thumbnail_key" = '', "thumbnail_asset_id" = NULL;
-- Modify "styles" table
ALTER TABLE "imageai"."styles" ALTER COLUMN "thumbnail_asset_id" TYPE uuid USING "thumbnail_asset_id"::uuid;
-- Drop "media_ref_sync" table
DROP TABLE "imageai"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "imageai"."retired_assets";
