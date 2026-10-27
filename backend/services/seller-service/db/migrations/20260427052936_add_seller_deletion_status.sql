-- Modify "sellers" table
ALTER TABLE "sellers" ADD COLUMN "status" character varying(20) NOT NULL DEFAULT 'active', ADD COLUMN "pii_cleared_at" timestamptz NULL, ADD COLUMN "payuni_account_unlinked_at" timestamptz NULL;
