-- Create "merchant_verification_outbox" table
CREATE TABLE "merchant_verification_outbox" (
  "id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "event_type" character varying(30) NOT NULL,
  "reason" character varying(200) NULL,
  "payload" jsonb NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL,
  "last_error" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "merchant_verification_outbox_pending_idx" to table: "merchant_verification_outbox"
CREATE INDEX "merchant_verification_outbox_pending_idx" ON "merchant_verification_outbox" ("status", "next_attempt_at") WHERE ((status)::text = 'pending'::text);
