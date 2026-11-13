-- Modify "accounts" table
ALTER TABLE "payment"."accounts" DROP CONSTRAINT "accounts_contact_email_check", DROP COLUMN "contact_email", ADD COLUMN "purge_pending_at" timestamptz NULL, ADD CONSTRAINT "accounts_purge_pending_check" CHECK ((purge_pending_at IS NULL) OR (external_account_id IS NOT NULL));
-- Create index "accounts_purge_pending" to table: "accounts"
CREATE INDEX "accounts_purge_pending" ON "payment"."accounts" ("store_id") WHERE (purge_pending_at IS NOT NULL);
