-- Existing snapshot asset IDs may be Java media TSIDs and cannot be read as UUIDs; the system has not launched, so drop the non-UUID entries.
UPDATE "moderation"."case_snapshots" SET "asset_ids" = ARRAY(SELECT a FROM unnest("asset_ids") AS a WHERE a ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$') WHERE "asset_ids" <> '{}';
-- Modify "case_snapshots" table
ALTER TABLE "moderation"."case_snapshots" ALTER COLUMN "asset_ids" DROP DEFAULT;
ALTER TABLE "moderation"."case_snapshots" ALTER COLUMN "asset_ids" TYPE uuid[] USING "asset_ids"::uuid[];
ALTER TABLE "moderation"."case_snapshots" ALTER COLUMN "asset_ids" SET DEFAULT '{}';
-- Drop "media_ref_sync" table
DROP TABLE "moderation"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "moderation"."retired_assets";
