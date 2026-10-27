-- Modify "accounts" table
ALTER TABLE "identity"."accounts" ADD CONSTRAINT "accounts_deletion_pending_check" CHECK ((deleted_at IS NULL) OR (deletion_pending_at IS NULL)), ADD COLUMN "deletion_pending_at" timestamptz NULL;
