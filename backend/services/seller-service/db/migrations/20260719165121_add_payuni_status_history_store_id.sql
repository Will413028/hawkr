-- Modify "payuni_status_history" table
ALTER TABLE "payuni_status_history" ADD COLUMN "store_id" bigint;

-- Backfill: every existing history row was recorded against a payuni_mer_id
-- that (if the registration still exists) already has the correct store_id
-- in payuni_registrations. Join back through that shared key.
UPDATE "payuni_status_history" h
SET "store_id" = r."store_id"
FROM "payuni_registrations" r
WHERE h."payuni_mer_id" = r."payuni_mer_id"
  AND h."store_id" IS NULL;

UPDATE "payuni_status_history"
SET "store_id" = "account_id"
WHERE "store_id" IS NULL;

ALTER TABLE "payuni_status_history" ALTER COLUMN "store_id" SET NOT NULL;
