-- Modify "commodity" table
ALTER TABLE "commodity" ADD COLUMN "canonical_unit" text NULL;
-- Modify "commodity_observation" table
ALTER TABLE "commodity_observation" ADD COLUMN "canonical_price" numeric(12,4) NULL;
