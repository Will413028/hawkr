-- Drop index "suspensions_admin_active" from table: "suspensions"
DROP INDEX "seller"."suspensions_admin_active";
-- Modify "suspensions" table
ALTER TABLE "seller"."suspensions" ADD COLUMN "account_id" bigint NULL;
-- Backfill "account_id" from the store of each existing suspension
UPDATE "seller"."suspensions" AS s SET "account_id" = st."account_id" FROM "seller"."stores" AS st WHERE st."store_id" = s."store_id";
-- Set "account_id" NOT NULL
ALTER TABLE "seller"."suspensions" ALTER COLUMN "account_id" SET NOT NULL;
-- Stop here with a clear message if one account already has two active admin suspensions
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM "seller"."suspensions"
        WHERE "source" = 'ADMIN' AND "lifted_at" IS NULL
        GROUP BY "account_id" HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'an account has two active admin suspensions; lift the extra one before applying this migration';
    END IF;
END $$;
-- Create index "suspensions_admin_active" to table: "suspensions"
CREATE UNIQUE INDEX "suspensions_admin_active" ON "seller"."suspensions" ("account_id") WHERE ((source = 'ADMIN'::text) AND (lifted_at IS NULL));
-- Create index "suspensions_account" to table: "suspensions"
CREATE INDEX "suspensions_account" ON "seller"."suspensions" ("account_id", "suspended_at" DESC, "suspension_id" DESC);
-- Drop index "suspensions_store" from table: "suspensions"
DROP INDEX "seller"."suspensions_store";
-- Create index "suspensions_store" to table: "suspensions"
CREATE INDEX "suspensions_store" ON "seller"."suspensions" ("store_id");
