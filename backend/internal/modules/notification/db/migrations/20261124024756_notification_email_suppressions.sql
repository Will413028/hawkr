-- Create "email_suppressions" table
CREATE TABLE "notification"."email_suppressions" (
  "address" text NOT NULL,
  "reason" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("address"),
  CONSTRAINT "email_suppressions_address_check" CHECK ((address = lower(address)) AND ((char_length(address) >= 3) AND (char_length(address) <= 254))),
  CONSTRAINT "email_suppressions_reason_check" CHECK (reason = ANY (ARRAY['BOUNCE'::text, 'COMPLAINT'::text]))
);
