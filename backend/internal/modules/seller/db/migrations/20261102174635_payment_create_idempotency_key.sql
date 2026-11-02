-- Drop index "payment_accounts_poll" from table: "payment_accounts"
DROP INDEX "seller"."payment_accounts_poll";
-- Modify "payment_accounts" table
ALTER TABLE "seller"."payment_accounts" ADD COLUMN "create_idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid();
-- Create index "payment_accounts_poll" to table: "payment_accounts"
CREATE INDEX "payment_accounts_poll" ON "seller"."payment_accounts" ("fetched_at" NULLS FIRST, "store_id") WHERE (status = ANY (ARRAY['UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text, 'DISABLED'::text]));
