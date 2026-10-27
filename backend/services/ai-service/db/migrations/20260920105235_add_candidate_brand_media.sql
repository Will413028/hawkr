-- Modify "procurement_candidates" table
ALTER TABLE "procurement_candidates" ADD COLUMN "brand_logo_key" text NOT NULL DEFAULT '', ADD COLUMN "brand_cover_key" text NOT NULL DEFAULT '';
