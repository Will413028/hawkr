-- Existing banner asset IDs are Java media TSIDs and cannot be read as UUIDs, and a banner requires both images; the system has not launched, so remove the old banners.
DELETE FROM "content"."banners";
-- Modify "banners" table
ALTER TABLE "content"."banners" ALTER COLUMN "preview_image_asset_id" TYPE uuid USING "preview_image_asset_id"::uuid, ALTER COLUMN "popup_image_asset_id" TYPE uuid USING "popup_image_asset_id"::uuid;
-- Drop "media_ref_sync" table
DROP TABLE "content"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "content"."retired_assets";
