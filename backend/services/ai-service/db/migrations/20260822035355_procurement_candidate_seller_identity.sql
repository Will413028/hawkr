-- Modify "procurement_candidates" table
ALTER TABLE "procurement_candidates" ADD COLUMN "store_id" bigint NULL, ADD COLUMN "seller_name" text NOT NULL DEFAULT '', ADD COLUMN "logo_key" text NOT NULL DEFAULT '', ADD COLUMN "store_code" text NOT NULL DEFAULT '';
