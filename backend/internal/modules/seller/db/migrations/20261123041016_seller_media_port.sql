-- Existing logo and cover asset IDs are Java media TSIDs and cannot be read as UUIDs; the system has not launched, so clear the images (the owner-side check requires key and asset ID to be NULL together).
UPDATE "seller"."stores" SET "logo_key" = NULL, "logo_asset_id" = NULL, "cover_key" = NULL, "cover_asset_id" = NULL WHERE "logo_asset_id" IS NOT NULL OR "cover_asset_id" IS NOT NULL;
UPDATE "seller"."prospects" SET "logo_key" = NULL, "logo_asset_id" = NULL, "logo_source_url" = NULL, "cover_key" = NULL, "cover_asset_id" = NULL, "cover_source_url" = NULL WHERE "logo_asset_id" IS NOT NULL OR "cover_asset_id" IS NOT NULL;
-- The remaining asset ID columns keep their rows: UUID text is cast as is, any other text (a Java media TSID) maps to a stable UUID derived from its md5, so every table that mentions the same old asset still agrees. Those assets do not exist in the Go media module; reading one reports a missing file.
-- Modify "case_inquiry_reply_evidence" table
ALTER TABLE "seller"."case_inquiry_reply_evidence" DROP CONSTRAINT "case_inquiry_reply_evidence_asset_check", ADD CONSTRAINT "case_inquiry_reply_evidence_asset_check" CHECK (object_key <> ''::text), ALTER COLUMN "asset_id" TYPE uuid USING (CASE WHEN "asset_id" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN "asset_id"::uuid ELSE md5("asset_id")::uuid END);
-- Modify "kyc_document_accesses" table
ALTER TABLE "seller"."kyc_document_accesses" DROP CONSTRAINT "kyc_document_accesses_asset_check", ALTER COLUMN "asset_id" TYPE uuid USING (CASE WHEN "asset_id" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN "asset_id"::uuid ELSE md5("asset_id")::uuid END);
-- Modify "kyc_documents" table
ALTER TABLE "seller"."kyc_documents" DROP CONSTRAINT "kyc_documents_asset_check", ADD CONSTRAINT "kyc_documents_asset_check" CHECK (object_key <> ''::text), ALTER COLUMN "asset_id" TYPE uuid USING (CASE WHEN "asset_id" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN "asset_id"::uuid ELSE md5("asset_id")::uuid END);
-- Modify "kyc_review_issues" table
ALTER TABLE "seller"."kyc_review_issues" DROP CONSTRAINT "kyc_review_issues_asset_check", ADD CONSTRAINT "kyc_review_issues_asset_check" CHECK ((document_asset_id IS NULL) OR (field = 'business.company_documents'::text)), ALTER COLUMN "document_asset_id" TYPE uuid USING (CASE WHEN "document_asset_id" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN "document_asset_id"::uuid ELSE md5("document_asset_id")::uuid END);
-- Modify "measure_request_evidence" table
ALTER TABLE "seller"."measure_request_evidence" DROP CONSTRAINT "measure_request_evidence_asset_check", ADD CONSTRAINT "measure_request_evidence_asset_check" CHECK (object_key <> ''::text), ALTER COLUMN "asset_id" TYPE uuid USING (CASE WHEN "asset_id" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN "asset_id"::uuid ELSE md5("asset_id")::uuid END);
-- Modify "prospects" table
ALTER TABLE "seller"."prospects" ALTER COLUMN "logo_asset_id" TYPE uuid USING NULL, ALTER COLUMN "cover_asset_id" TYPE uuid USING NULL;
-- Modify "report_evidence" table
ALTER TABLE "seller"."report_evidence" DROP CONSTRAINT "report_evidence_asset_check", ADD CONSTRAINT "report_evidence_asset_check" CHECK (object_key <> ''::text), ALTER COLUMN "asset_id" TYPE uuid USING (CASE WHEN "asset_id" ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' THEN "asset_id"::uuid ELSE md5("asset_id")::uuid END);
-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_images_check", ADD CONSTRAINT "stores_images_check" CHECK ((logo_key <> ''::text) AND (cover_key <> ''::text)), ALTER COLUMN "logo_asset_id" TYPE uuid USING NULL, ALTER COLUMN "cover_asset_id" TYPE uuid USING NULL;
-- Drop "media_ref_sync" table
DROP TABLE "seller"."media_ref_sync";
-- Drop "retired_assets" table
DROP TABLE "seller"."retired_assets";
