-- Create "email_suppressions" table
CREATE TABLE "notification"."email_suppressions" (
  "address_hash" bytea NOT NULL,
  "reason" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("address_hash"),
  CONSTRAINT "email_suppressions_address_hash_check" CHECK (octet_length(address_hash) = 32),
  CONSTRAINT "email_suppressions_reason_check" CHECK (reason = ANY (ARRAY['HARD_BOUNCE'::text, 'COMPLAINT'::text]))
);
