-- Add new schema named "payment"
CREATE SCHEMA "payment";
-- Create "account_mapping_deletions" table
CREATE TABLE "payment"."account_mapping_deletions" (
  "id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "store_id" bigint NOT NULL,
  "provider" text NOT NULL,
  "external_account_id" text NULL,
  "admin_id" bigint NOT NULL,
  "deleted_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "account_mapping_deletions_admin_id_check" CHECK (admin_id > 0),
  CONSTRAINT "account_mapping_deletions_external_id_check" CHECK ((external_account_id IS NULL) OR ((char_length(external_account_id) >= 1) AND (char_length(external_account_id) <= 255))),
  CONSTRAINT "account_mapping_deletions_store_id_check" CHECK (store_id > 0)
);
-- Create index "account_mapping_deletions_store" to table: "account_mapping_deletions"
CREATE INDEX "account_mapping_deletions_store" ON "payment"."account_mapping_deletions" ("store_id", "deleted_at");
-- Create "accounts" table
CREATE TABLE "payment"."accounts" (
  "store_id" bigint NOT NULL,
  "provider" text NOT NULL,
  "external_account_id" text NULL,
  "requirement_collection" text NOT NULL,
  "status" text NOT NULL DEFAULT 'NOT_CREATED',
  "transfers_active" boolean NOT NULL DEFAULT false,
  "payouts_active" boolean NOT NULL DEFAULT false,
  "requirements" jsonb NOT NULL DEFAULT '{}',
  "status_reason" text NULL,
  "fetched_at" timestamptz NULL,
  "create_failed_at" timestamptz NULL,
  "create_called_at" timestamptz NULL,
  "create_completed_at" timestamptz NULL,
  "create_idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "seller_read_claimed_at" timestamptz NULL,
  "purge_pending_at" timestamptz NULL,
  "purge_blocked_at" timestamptz NULL,
  "purge_blocked_reason" text NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "accounts_called_check" CHECK ((external_account_id IS NULL) OR (create_called_at IS NOT NULL)),
  CONSTRAINT "accounts_collection_check" CHECK (requirement_collection = ANY (ARRAY['PROVIDER'::text, 'PLATFORM'::text])),
  CONSTRAINT "accounts_completed_check" CHECK ((create_completed_at IS NULL) OR (external_account_id IS NOT NULL)),
  CONSTRAINT "accounts_create_failed_check" CHECK ((create_failed_at IS NULL) OR (create_completed_at IS NULL)),
  CONSTRAINT "accounts_created_check" CHECK ((status = 'NOT_CREATED'::text) = (create_completed_at IS NULL)),
  CONSTRAINT "accounts_enabled_check" CHECK ((status = 'ENABLED'::text) = transfers_active),
  CONSTRAINT "accounts_external_id_check" CHECK ((external_account_id IS NULL) OR ((char_length(external_account_id) >= 1) AND (char_length(external_account_id) <= 255))),
  CONSTRAINT "accounts_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text),
  CONSTRAINT "accounts_purge_blocked_check" CHECK (((purge_blocked_at IS NULL) = (purge_blocked_reason IS NULL)) AND ((purge_blocked_at IS NULL) OR (purge_pending_at IS NOT NULL))),
  CONSTRAINT "accounts_purge_blocked_reason_check" CHECK ((purge_blocked_reason IS NULL) OR ((char_length(purge_blocked_reason) >= 1) AND (char_length(purge_blocked_reason) <= 200))),
  CONSTRAINT "accounts_requirements_check" CHECK (jsonb_typeof(requirements) = 'object'::text),
  CONSTRAINT "accounts_snapshot_check" CHECK ((create_completed_at IS NOT NULL) OR ((fetched_at IS NULL) AND (NOT payouts_active) AND (status_reason IS NULL))),
  CONSTRAINT "accounts_status_check" CHECK (status = ANY (ARRAY['NOT_CREATED'::text, 'UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text, 'ENABLED'::text, 'DISABLED'::text])),
  CONSTRAINT "accounts_status_reason_check" CHECK ((status_reason IS NULL) OR ((char_length(status_reason) >= 1) AND (char_length(status_reason) <= 200))),
  CONSTRAINT "accounts_store_id_check" CHECK (store_id > 0)
);
-- Create index "accounts_create_failed" to table: "accounts"
CREATE INDEX "accounts_create_failed" ON "payment"."accounts" ("create_failed_at") WHERE (create_failed_at IS NOT NULL);
-- Create index "accounts_external" to table: "accounts"
CREATE UNIQUE INDEX "accounts_external" ON "payment"."accounts" ("provider", "external_account_id") WHERE (external_account_id IS NOT NULL);
-- Create index "accounts_poll" to table: "accounts"
CREATE INDEX "accounts_poll" ON "payment"."accounts" ("store_id") WHERE (status = ANY (ARRAY['UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text, 'DISABLED'::text]));
-- Create index "accounts_purge_blocked" to table: "accounts"
CREATE INDEX "accounts_purge_blocked" ON "payment"."accounts" ("purge_blocked_at") WHERE (purge_blocked_at IS NOT NULL);
-- Create index "accounts_purge_pending" to table: "accounts"
CREATE INDEX "accounts_purge_pending" ON "payment"."accounts" ("store_id") WHERE ((purge_pending_at IS NOT NULL) AND (purge_blocked_at IS NULL));
