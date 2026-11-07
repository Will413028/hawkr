-- Create "payment_provider_events" table
CREATE TABLE "seller"."payment_provider_events" (
  "provider" text NOT NULL,
  "event_id" text NOT NULL,
  "external_account_id" text NOT NULL,
  "received_at" timestamptz NOT NULL,
  "processed_at" timestamptz NULL,
  PRIMARY KEY ("provider", "event_id"),
  CONSTRAINT "payment_provider_events_account_check" CHECK ((char_length(external_account_id) >= 1) AND (char_length(external_account_id) <= 255)),
  CONSTRAINT "payment_provider_events_event_id_check" CHECK ((char_length(event_id) >= 1) AND (char_length(event_id) <= 255)),
  CONSTRAINT "payment_provider_events_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text)
);
-- Create index "payment_provider_events_pending" to table: "payment_provider_events"
CREATE INDEX "payment_provider_events_pending" ON "seller"."payment_provider_events" ("provider", "external_account_id") WHERE (processed_at IS NULL);
-- Create "payment_accounts" table
CREATE TABLE "seller"."payment_accounts" (
  "store_id" bigint NOT NULL,
  "provider" text NOT NULL,
  "external_account_id" text NULL,
  "requirement_collection" text NOT NULL,
  "status" text NOT NULL DEFAULT 'NOT_CREATED',
  "transfers_active" boolean NOT NULL DEFAULT false,
  "payouts_active" boolean NOT NULL DEFAULT false,
  "requirements" jsonb NOT NULL DEFAULT '{}',
  "disabled_reason" text NULL,
  "fetched_at" timestamptz NULL,
  "create_failed_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "payment_accounts_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payment_accounts_collection_check" CHECK (requirement_collection = ANY (ARRAY['PROVIDER'::text, 'PLATFORM'::text])),
  CONSTRAINT "payment_accounts_create_failed_check" CHECK ((create_failed_at IS NULL) OR (external_account_id IS NULL)),
  CONSTRAINT "payment_accounts_created_check" CHECK ((status = 'NOT_CREATED'::text) = (external_account_id IS NULL)),
  CONSTRAINT "payment_accounts_disabled_reason_check" CHECK ((disabled_reason IS NULL) OR ((char_length(disabled_reason) >= 1) AND (char_length(disabled_reason) <= 200))),
  CONSTRAINT "payment_accounts_enabled_check" CHECK ((status = 'ENABLED'::text) = transfers_active),
  CONSTRAINT "payment_accounts_external_id_check" CHECK ((external_account_id IS NULL) OR ((char_length(external_account_id) >= 1) AND (char_length(external_account_id) <= 255))),
  CONSTRAINT "payment_accounts_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text),
  CONSTRAINT "payment_accounts_requirements_check" CHECK (jsonb_typeof(requirements) = 'object'::text),
  CONSTRAINT "payment_accounts_snapshot_check" CHECK ((external_account_id IS NOT NULL) OR ((fetched_at IS NULL) AND (NOT payouts_active) AND (disabled_reason IS NULL))),
  CONSTRAINT "payment_accounts_status_check" CHECK (status = ANY (ARRAY['NOT_CREATED'::text, 'ONBOARDING'::text, 'UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text, 'ENABLED'::text, 'DISABLED'::text]))
);
-- Create index "payment_accounts_external" to table: "payment_accounts"
CREATE UNIQUE INDEX "payment_accounts_external" ON "seller"."payment_accounts" ("provider", "external_account_id") WHERE (external_account_id IS NOT NULL);
-- Create index "payment_accounts_poll" to table: "payment_accounts"
CREATE INDEX "payment_accounts_poll" ON "seller"."payment_accounts" ("fetched_at" NULLS FIRST, "store_id") WHERE (status = ANY (ARRAY['ONBOARDING'::text, 'UNDER_REVIEW'::text, 'ACTION_REQUIRED'::text]));
