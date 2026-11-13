-- Modify "accounts" table
ALTER TABLE "payment"."accounts" DROP CONSTRAINT "accounts_contact_email_check", DROP COLUMN "contact_email", ADD COLUMN "close_deferred_at" timestamptz NULL, ADD CONSTRAINT "accounts_close_deferred_check" CHECK ((close_deferred_at IS NULL) OR (external_account_id IS NOT NULL));
-- Create index "accounts_close_deferred" to table: "accounts"
CREATE INDEX "accounts_close_deferred" ON "payment"."accounts" ("store_id") WHERE (close_deferred_at IS NOT NULL);
