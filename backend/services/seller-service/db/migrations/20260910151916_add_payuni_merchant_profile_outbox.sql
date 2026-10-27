-- Create "payuni_merchant_profile_outbox" table
CREATE TABLE "payuni_merchant_profile_outbox" (
  "id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "payload" jsonb NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL,
  "last_error" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "payuni_merchant_profile_outbox_pending_idx" to table: "payuni_merchant_profile_outbox"
CREATE INDEX "payuni_merchant_profile_outbox_pending_idx" ON "payuni_merchant_profile_outbox" ("status", "next_attempt_at") WHERE ((status)::text = 'pending'::text);
