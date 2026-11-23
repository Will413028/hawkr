-- Add new schema named "notification"
CREATE SCHEMA "notification";
-- Create "device_tokens" table
CREATE TABLE "notification"."device_tokens" (
  "token" text NOT NULL,
  "account_id" bigint NOT NULL,
  "app" text NOT NULL,
  "last_registered_at" timestamptz NOT NULL,
  PRIMARY KEY ("token"),
  CONSTRAINT "device_tokens_account_check" CHECK (account_id > 0),
  CONSTRAINT "device_tokens_app_check" CHECK (app = ANY (ARRAY['BUYER'::text, 'SELLER'::text])),
  CONSTRAINT "device_tokens_token_check" CHECK ((char_length(token) >= 1) AND (char_length(token) <= 4096))
);
-- Create index "device_tokens_account" to table: "device_tokens"
CREATE INDEX "device_tokens_account" ON "notification"."device_tokens" ("account_id", "app");
-- Create index "device_tokens_last_registered" to table: "device_tokens"
CREATE INDEX "device_tokens_last_registered" ON "notification"."device_tokens" ("last_registered_at");
-- Create "intents" table
CREATE TABLE "notification"."intents" (
  "id" uuid NOT NULL,
  "type" text NOT NULL,
  "idempotency_key" text NOT NULL,
  "params" jsonb NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "intents_idempotency_key_key" UNIQUE ("idempotency_key"),
  CONSTRAINT "intents_idempotency_key_check" CHECK ((char_length(idempotency_key) >= 1) AND (char_length(idempotency_key) <= 200)),
  CONSTRAINT "intents_type_check" CHECK (type ~ '^[a-z][a-z0-9_]{0,62}$'::text)
);
-- Create index "intents_created" to table: "intents"
CREATE INDEX "intents_created" ON "notification"."intents" ("created_at");
-- Create "preference_overrides" table
CREATE TABLE "notification"."preference_overrides" (
  "account_id" bigint NOT NULL,
  "audience" text NOT NULL,
  "category" text NOT NULL,
  "channel" text NOT NULL,
  "enabled" boolean NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("account_id", "audience", "category", "channel"),
  CONSTRAINT "preference_overrides_account_check" CHECK (account_id > 0),
  CONSTRAINT "preference_overrides_audience_check" CHECK (audience = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "preference_overrides_category_check" CHECK (category ~ '^[A-Z][A-Z0-9_]{0,62}$'::text),
  CONSTRAINT "preference_overrides_channel_check" CHECK (channel = ANY (ARRAY['INBOX'::text, 'PUSH'::text, 'EMAIL'::text]))
);
-- Create "deliveries" table
CREATE TABLE "notification"."deliveries" (
  "id" uuid NOT NULL,
  "intent_id" uuid NOT NULL,
  "account_id" bigint NOT NULL,
  "audience" text NOT NULL,
  "store_id" bigint NULL,
  "channel" text NOT NULL,
  "status" text NOT NULL,
  "locale" text NULL,
  "sent_title" text NULL,
  "sent_body" text NULL,
  "failure_reason" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "sent_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "deliveries_recipient_channel_key" UNIQUE ("intent_id", "account_id", "audience", "channel"),
  CONSTRAINT "deliveries_intent_id_fkey" FOREIGN KEY ("intent_id") REFERENCES "notification"."intents" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "deliveries_account_check" CHECK (account_id > 0),
  CONSTRAINT "deliveries_audience_check" CHECK (audience = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "deliveries_channel_check" CHECK (channel = ANY (ARRAY['PUSH'::text, 'EMAIL'::text])),
  CONSTRAINT "deliveries_failure_check" CHECK ((status = 'FAILED_PERMANENT'::text) = (failure_reason IS NOT NULL)),
  CONSTRAINT "deliveries_locale_check" CHECK ((locale IS NULL) OR (locale ~ '^[a-z]{2,3}(-[A-Z]{2})?$'::text)),
  CONSTRAINT "deliveries_sent_check" CHECK ((status = 'SENT'::text) = (sent_at IS NOT NULL)),
  CONSTRAINT "deliveries_sent_text_check" CHECK ((status = 'SENT'::text) OR ((locale IS NULL) AND (sent_title IS NULL) AND (sent_body IS NULL))),
  CONSTRAINT "deliveries_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'SENT'::text, 'FAILED_PERMANENT'::text])),
  CONSTRAINT "deliveries_store_check" CHECK (((audience = 'STORE'::text) = (store_id IS NOT NULL)) AND ((store_id IS NULL) OR (store_id > 0)))
);
-- Create index "deliveries_created" to table: "deliveries"
CREATE INDEX "deliveries_created" ON "notification"."deliveries" ("created_at");
-- Create index "deliveries_recipient" to table: "deliveries"
CREATE INDEX "deliveries_recipient" ON "notification"."deliveries" ("account_id", "audience");
-- Create index "deliveries_status" to table: "deliveries"
CREATE INDEX "deliveries_status" ON "notification"."deliveries" ("status", "created_at") WHERE (status <> 'SENT'::text);
-- Create "inbox_items" table
CREATE TABLE "notification"."inbox_items" (
  "id" uuid NOT NULL,
  "account_id" bigint NOT NULL,
  "audience" text NOT NULL,
  "store_id" bigint NULL,
  "type" text NOT NULL,
  "params" jsonb NOT NULL,
  "aggregation_key" text NULL,
  "actor_account_ids" bigint[] NOT NULL DEFAULT '{}',
  "intent_id" uuid NOT NULL,
  "read_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "inbox_items_intent_id_fkey" FOREIGN KEY ("intent_id") REFERENCES "notification"."intents" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "inbox_items_account_check" CHECK (account_id > 0),
  CONSTRAINT "inbox_items_aggregation_key_check" CHECK ((aggregation_key IS NULL) OR ((char_length(aggregation_key) >= 1) AND (char_length(aggregation_key) <= 200))),
  CONSTRAINT "inbox_items_audience_check" CHECK (audience = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "inbox_items_store_check" CHECK (((audience = 'STORE'::text) = (store_id IS NOT NULL)) AND ((store_id IS NULL) OR (store_id > 0))),
  CONSTRAINT "inbox_items_type_check" CHECK (type ~ '^[a-z][a-z0-9_]{0,62}$'::text),
  CONSTRAINT "inbox_items_updated_check" CHECK (updated_at >= created_at)
);
-- Create index "inbox_items_recipient" to table: "inbox_items"
CREATE INDEX "inbox_items_recipient" ON "notification"."inbox_items" ("account_id", "audience", "updated_at" DESC, "id" DESC);
-- Create index "inbox_items_unread" to table: "inbox_items"
CREATE INDEX "inbox_items_unread" ON "notification"."inbox_items" ("account_id", "audience") WHERE (read_at IS NULL);
-- Create index "inbox_items_unread_aggregation" to table: "inbox_items"
CREATE UNIQUE INDEX "inbox_items_unread_aggregation" ON "notification"."inbox_items" ("account_id", "audience", "aggregation_key") WHERE ((aggregation_key IS NOT NULL) AND (read_at IS NULL));
-- Create index "inbox_items_updated" to table: "inbox_items"
CREATE INDEX "inbox_items_updated" ON "notification"."inbox_items" ("updated_at");
