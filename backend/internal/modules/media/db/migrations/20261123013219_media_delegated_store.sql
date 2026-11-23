-- Modify "staged_uploads" table
ALTER TABLE "media"."staged_uploads" ADD CONSTRAINT "staged_uploads_delegated_store_check" CHECK ((delegated_store_id IS NULL) OR ((delegated_store_id > 0) AND uploader_admin)), ADD COLUMN "delegated_store_id" bigint NULL;
