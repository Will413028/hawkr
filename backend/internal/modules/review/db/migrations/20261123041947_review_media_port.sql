-- Existing review image asset IDs are Java media TSIDs and cannot be read as UUIDs; the system has not launched, so remove the old attachments (the reviews stay).
DELETE FROM "review"."review_images";
-- Modify "review_images" table
ALTER TABLE "review"."review_images" DROP CONSTRAINT "review_images_asset_check", ALTER COLUMN "asset_id" TYPE uuid USING "asset_id"::uuid;
-- Drop "media_ref_sync" table
DROP TABLE "review"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "review"."retired_assets";
