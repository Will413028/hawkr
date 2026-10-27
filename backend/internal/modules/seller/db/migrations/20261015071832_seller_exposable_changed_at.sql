-- Rename column "announced_at" to "exposable_changed_at" in "store_exposures" table
ALTER TABLE "seller"."store_exposures" RENAME COLUMN "announced_at" TO "exposable_changed_at";
-- Rename constraint "store_exposures_announced_check" to "store_exposures_exposable_changed_check"
ALTER TABLE "seller"."store_exposures" RENAME CONSTRAINT "store_exposures_announced_check" TO "store_exposures_exposable_changed_check";
