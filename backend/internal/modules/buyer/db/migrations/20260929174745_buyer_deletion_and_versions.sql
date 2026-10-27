-- Modify "buyers" table
ALTER TABLE "buyer"."buyers" ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Modify "membership_plans" table
ALTER TABLE "buyer"."membership_plans" ALTER COLUMN "monthly_price_twd" TYPE bigint, ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Modify "shipping_addresses" table
ALTER TABLE "buyer"."shipping_addresses" ADD COLUMN "version" bigint NOT NULL DEFAULT 1;
-- Drop "deletion_audits" table
DROP TABLE "buyer"."deletion_audits";
