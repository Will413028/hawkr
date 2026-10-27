CREATE SEQUENCE "payuni_merchant_code_seq" START WITH 100000000000;

-- Modify "payuni_registrations" table
ALTER TABLE "payuni_registrations" ADD COLUMN "merchant_code" bigint NULL;

-- Backfill: every existing row whose store_id already fits PayUni's cap
-- (the pre-rekey grandfather scheme, <= 10 digits) keeps merchant_code ==
-- store_id, preserving its exact already-approved MerID unchanged.
UPDATE "payuni_registrations" SET "merchant_code" = "store_id" WHERE "store_id" <= 9999999999;

-- Any remaining row (a long-TSID store that already has a row — e.g. a
-- failed first bind attempt from before this migration shipped) gets a
-- fresh code assigned now, so a future retry finds and reuses it instead of
-- resolving inconsistently across attempts.
UPDATE "payuni_registrations" SET "merchant_code" = nextval('payuni_merchant_code_seq') WHERE "merchant_code" IS NULL;

ALTER TABLE "payuni_registrations" ALTER COLUMN "merchant_code" SET NOT NULL;
