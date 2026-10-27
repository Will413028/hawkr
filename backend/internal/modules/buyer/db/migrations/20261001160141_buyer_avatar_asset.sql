-- Modify "buyers" table
ALTER TABLE "buyer"."buyers" ADD COLUMN "avatar_asset_id" text NULL;
-- Existing avatars have no recorded asset ID; clear them so the pairing check holds (the client shows the default avatar).
UPDATE "buyer"."buyers" SET "avatar" = NULL WHERE "avatar" IS NOT NULL AND "avatar_asset_id" IS NULL;
-- Pair the avatar key with its asset ID
ALTER TABLE "buyer"."buyers" ADD CONSTRAINT "buyers_avatar_asset_check" CHECK ((avatar IS NULL) = (avatar_asset_id IS NULL));
