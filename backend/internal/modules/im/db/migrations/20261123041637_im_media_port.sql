-- Existing image asset IDs are Java media TSIDs and cannot be read as UUIDs; the system has not launched, so turn existing image messages into erased tombstones.
UPDATE "im"."messages" SET "image_asset_id" = NULL, "image_object_key" = NULL, "erased_at" = now() WHERE "image_asset_id" IS NOT NULL;
-- Modify "messages" table
ALTER TABLE "im"."messages" DROP CONSTRAINT "messages_image_check", ADD CONSTRAINT "messages_image_check" CHECK ((image_object_key IS NULL) OR (image_object_key <> ''::text)), ALTER COLUMN "image_asset_id" TYPE uuid USING "image_asset_id"::uuid;
-- Drop "media_ref_sync" table
DROP TABLE "im"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "im"."retired_assets";
