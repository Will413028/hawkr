-- Create "payuni_observations" table
CREATE TABLE "seller"."payuni_observations" (
  "observation_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "source" text NOT NULL,
  "event_key" text NULL,
  "mer_status" text NULL,
  "member_status" text NULL,
  "credit_status" text NULL,
  "applepay_status" text NULL,
  "googlepay_status" text NULL,
  "atm_status" text NULL,
  "reason" text NULL,
  "executed_at" timestamptz NOT NULL,
  "received_at" timestamptz NOT NULL,
  PRIMARY KEY ("observation_id"),
  CONSTRAINT "payuni_observations_event_key" UNIQUE ("store_id", "event_key"),
  CONSTRAINT "payuni_observations_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payuni_observations_event_key_check" CHECK ((char_length(event_key) >= 1) AND (char_length(event_key) <= 200)),
  CONSTRAINT "payuni_observations_source_check" CHECK (source = ANY (ARRAY['WEBHOOK'::text, 'POLL'::text, 'REFRESH'::text, 'BIND'::text])),
  CONSTRAINT "payuni_observations_status_codes_check" CHECK ((char_length(mer_status) >= 1) AND (char_length(mer_status) <= 20) AND (char_length(member_status) >= 1) AND (char_length(member_status) <= 20) AND (char_length(credit_status) >= 1) AND (char_length(credit_status) <= 20) AND (char_length(applepay_status) >= 1) AND (char_length(applepay_status) <= 20) AND (char_length(googlepay_status) >= 1) AND (char_length(googlepay_status) <= 20) AND (char_length(atm_status) >= 1) AND (char_length(atm_status) <= 20) AND (char_length(reason) >= 1) AND (char_length(reason) <= 500)),
  CONSTRAINT "payuni_observations_webhook_key_check" CHECK ((source <> 'WEBHOOK'::text) OR (event_key IS NOT NULL))
);
-- Create index "payuni_observations_store" to table: "payuni_observations"
CREATE INDEX "payuni_observations_store" ON "seller"."payuni_observations" ("store_id", "executed_at" DESC, "observation_id" DESC);
-- Create "payuni_registrations" table
CREATE TABLE "seller"."payuni_registrations" (
  "store_id" bigint NOT NULL,
  "mer_id" text NOT NULL,
  "application_id" uuid NOT NULL,
  "bind_status" text NOT NULL DEFAULT 'PENDING',
  "bind_error_code" text NULL,
  "bind_attempts" integer NOT NULL DEFAULT 0,
  "bound_at" timestamptz NULL,
  "contact_name" text NOT NULL,
  "contact_mobile" text NOT NULL,
  "contact_tel" text NULL,
  "mer_email" text NOT NULL,
  "mer_tel" text NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "intent_seq" bigint NOT NULL DEFAULT 1,
  "synced_seq" bigint NOT NULL DEFAULT 0,
  "sync_error_code" text NULL,
  "claimed_until" timestamptz NULL,
  "mer_status" text NULL,
  "member_status" text NULL,
  "credit_status" text NULL,
  "applepay_status" text NULL,
  "googlepay_status" text NULL,
  "atm_status" text NULL,
  "reason" text NULL,
  "outcome" text NOT NULL DEFAULT 'NONE',
  "status_observed_at" timestamptz NULL,
  "status_checked_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "payuni_registrations_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payuni_registrations_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payuni_registrations_attempts_check" CHECK (bind_attempts >= 0),
  CONSTRAINT "payuni_registrations_bind_error_check" CHECK (((bind_status = 'FAILED'::text) = (bind_error_code IS NOT NULL)) AND (bind_error_code ~ '^[A-Z][A-Z0-9_]{0,39}$'::text)),
  CONSTRAINT "payuni_registrations_bind_status_check" CHECK (bind_status = ANY (ARRAY['PENDING'::text, 'BOUND'::text, 'FAILED'::text])),
  CONSTRAINT "payuni_registrations_bound_check" CHECK ((bind_status = 'BOUND'::text) = (bound_at IS NOT NULL)),
  CONSTRAINT "payuni_registrations_contact_mobile_check" CHECK (contact_mobile ~ '^09[0-9]{8}$'::text),
  CONSTRAINT "payuni_registrations_contact_name_check" CHECK ((char_length(contact_name) >= 1) AND (char_length(contact_name) <= 100) AND (contact_name = btrim(contact_name))),
  CONSTRAINT "payuni_registrations_contact_tel_check" CHECK (contact_tel ~ '^0[0-9]{8,9}$'::text),
  CONSTRAINT "payuni_registrations_mer_email_check" CHECK ((char_length(mer_email) <= 254) AND (mer_email = lower(mer_email)) AND (mer_email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'::text)),
  CONSTRAINT "payuni_registrations_mer_id_check" CHECK (mer_id ~ '^HAWK[0-9]{1,10}$'::text),
  CONSTRAINT "payuni_registrations_mer_tel_check" CHECK (mer_tel ~ '^0[0-9]{8,9}$'::text),
  CONSTRAINT "payuni_registrations_observed_check" CHECK ((outcome = 'NONE'::text) = (status_observed_at IS NULL)),
  CONSTRAINT "payuni_registrations_outcome_check" CHECK (outcome = ANY (ARRAY['NONE'::text, 'UNDER_REVIEW'::text, 'PENDING_INFO'::text, 'APPROVED'::text, 'REJECTED'::text, 'UNRECOGNIZED'::text])),
  CONSTRAINT "payuni_registrations_seq_check" CHECK ((intent_seq >= 1) AND (synced_seq >= 0) AND (synced_seq <= intent_seq)),
  CONSTRAINT "payuni_registrations_status_codes_check" CHECK ((char_length(mer_status) >= 1) AND (char_length(mer_status) <= 20) AND (char_length(member_status) >= 1) AND (char_length(member_status) <= 20) AND (char_length(credit_status) >= 1) AND (char_length(credit_status) <= 20) AND (char_length(applepay_status) >= 1) AND (char_length(applepay_status) <= 20) AND (char_length(googlepay_status) >= 1) AND (char_length(googlepay_status) <= 20) AND (char_length(atm_status) >= 1) AND (char_length(atm_status) <= 20) AND (char_length(reason) >= 1) AND (char_length(reason) <= 500)),
  CONSTRAINT "payuni_registrations_sync_error_check" CHECK (sync_error_code ~ '^[A-Z][A-Z0-9_]{0,39}$'::text),
  CONSTRAINT "payuni_registrations_version_check" CHECK (version >= 1)
);
-- Create index "payuni_registrations_mer_id" to table: "payuni_registrations"
CREATE UNIQUE INDEX "payuni_registrations_mer_id" ON "seller"."payuni_registrations" ("mer_id");
-- Create index "payuni_registrations_poll" to table: "payuni_registrations"
CREATE INDEX "payuni_registrations_poll" ON "seller"."payuni_registrations" ("status_checked_at" NULLS FIRST, "store_id") WHERE ((bind_status = 'BOUND'::text) AND (outcome = ANY (ARRAY['NONE'::text, 'UNDER_REVIEW'::text, 'PENDING_INFO'::text, 'UNRECOGNIZED'::text])));
-- Create index "payuni_registrations_unsynced" to table: "payuni_registrations"
CREATE INDEX "payuni_registrations_unsynced" ON "seller"."payuni_registrations" ("updated_at", "store_id") WHERE ((bind_status <> 'FAILED'::text) AND (synced_seq < intent_seq));
