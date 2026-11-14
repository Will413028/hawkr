-- Add new schema named "payment"
CREATE SCHEMA "payment";
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
  "create_idempotency_key" uuid NOT NULL DEFAULT gen_random_uuid(),
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "seller_read_claimed_at" timestamptz NULL,
  "purge_pending_at" timestamptz NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "accounts_collection_check" CHECK (requirement_collection = ANY (ARRAY['PROVIDER'::text, 'PLATFORM'::text])),
  CONSTRAINT "accounts_create_failed_check" CHECK ((create_failed_at IS NULL) OR (external_account_id IS NULL)),
  CONSTRAINT "accounts_created_check" CHECK ((status = 'NOT_CREATED'::text) = (external_account_id IS NULL)),
  CONSTRAINT "accounts_enabled_check" CHECK ((status = 'ENABLED'::text) = transfers_active),
  CONSTRAINT "accounts_external_id_check" CHECK ((external_account_id IS NULL) OR ((char_length(external_account_id) >= 1) AND (char_length(external_account_id) <= 255))),
  CONSTRAINT "accounts_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text),
  CONSTRAINT "accounts_requirements_check" CHECK (jsonb_typeof(requirements) = 'object'::text),
  CONSTRAINT "accounts_snapshot_check" CHECK ((external_account_id IS NOT NULL) OR ((fetched_at IS NULL) AND (NOT payouts_active) AND (status_reason IS NULL))),
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
-- Create index "accounts_purge_pending" to table: "accounts"
CREATE INDEX "accounts_purge_pending" ON "payment"."accounts" ("store_id") WHERE (purge_pending_at IS NOT NULL);
