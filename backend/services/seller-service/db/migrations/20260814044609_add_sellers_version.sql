-- Modify "sellers" table
ALTER TABLE "sellers" ADD CONSTRAINT "chk_sellers_version_nonneg" CHECK (version >= 0), ADD COLUMN "version" bigint NOT NULL DEFAULT 0;
