-- Drop index "payment_accounts_poll" from table: "payment_accounts"
DROP INDEX "seller"."payment_accounts_poll";
-- Rename column "disabled_reason" to "status_reason"
ALTER TABLE "seller"."payment_accounts" RENAME COLUMN "disabled_reason" TO "status_reason";
-- Move rows off the removed status
UPDATE "seller"."payment_accounts" SET "status" = 'ACTION_REQUIRED' WHERE "status" = 'ONBOARDING';
-- Modify "payment_accounts" table
ALTER TABLE "seller"."payment_accounts" DROP CONSTRAINT "payment_accounts_disabled_reason_check", DROP CONSTRAINT "payment_accounts_snapshot_check", ADD CONSTRAINT "payment_accounts_snapshot_check" CHECK ((external_account_id IS NOT NULL) OR ((fetched_at IS NULL) AND (NOT payouts_active) AND (status_reason IS NULL))), DROP CONSTRAINT "payment_accounts_status_check", ADD CONSTRAINT "payment_accounts_status_check" CHECK (status = ANY (ARRAY['NOT_CREATED'::text, 'UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text, 'ENABLED'::text, 'DISABLED'::text])), ADD CONSTRAINT "payment_accounts_status_reason_check" CHECK ((status_reason IS NULL) OR ((char_length(status_reason) >= 1) AND (char_length(status_reason) <= 200)));
-- Create index "payment_accounts_poll" to table: "payment_accounts"
CREATE INDEX "payment_accounts_poll" ON "seller"."payment_accounts" ("fetched_at" NULLS FIRST, "store_id") WHERE (status = ANY (ARRAY['UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text]));
