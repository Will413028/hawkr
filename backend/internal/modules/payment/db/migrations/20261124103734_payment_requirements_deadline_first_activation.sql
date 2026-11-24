-- Modify "accounts" table
ALTER TABLE "payment"."accounts" ADD COLUMN "requirements_deadline" timestamptz NULL, ADD COLUMN "first_activated_at" timestamptz NULL;
UPDATE "payment"."accounts" SET "first_activated_at" = COALESCE("fetched_at", "updated_at") WHERE "transfers_active";
ALTER TABLE "payment"."accounts" ADD CONSTRAINT "accounts_deadline_snapshot_check" CHECK ((create_completed_at IS NOT NULL) OR ((requirements_deadline IS NULL) AND (first_activated_at IS NULL))), ADD CONSTRAINT "accounts_first_activated_check" CHECK ((NOT transfers_active) OR (first_activated_at IS NOT NULL));
