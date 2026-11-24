-- Existing avatar asset IDs are Java media TSIDs and cannot be read as UUIDs; the system has not launched, so clear them (the client shows the default avatar).
UPDATE "buyer"."buyers" SET "avatar" = NULL, "avatar_asset_id" = NULL WHERE "avatar_asset_id" IS NOT NULL;
-- Modify "buyers" table
ALTER TABLE "buyer"."buyers" ALTER COLUMN "avatar_asset_id" TYPE uuid USING NULL;
-- Drop "media_ref_sync" table
DROP TABLE "buyer"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "buyer"."retired_assets";
