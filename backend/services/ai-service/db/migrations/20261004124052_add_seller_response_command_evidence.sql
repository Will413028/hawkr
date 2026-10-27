-- Modify "procurement_seller_response_versions" table
ALTER TABLE "procurement_seller_response_versions" ADD CONSTRAINT "chk_prsrv_verification_all_or_none" CHECK (((verification_method IS NULL) AND (verification_email IS NULL) AND (verified_at IS NULL) AND (verification_session_id IS NULL)) OR ((verification_method = 'GOOGLE'::text) AND (verification_email <> ''::text) AND (verified_at IS NOT NULL) AND (verification_session_id <> ''::text))), ADD COLUMN "verification_method" text NULL, ADD COLUMN "verification_email" text NULL, ADD COLUMN "verified_at" timestamptz NULL, ADD COLUMN "verification_session_id" text NULL;
-- Create "procurement_seller_response_commands" table
CREATE TABLE "procurement_seller_response_commands" (
  "id" bigint NOT NULL,
  "seller_response_id" bigint NOT NULL,
  "kind" text NOT NULL,
  "withdrawn_version" integer NULL,
  "client_request_id" text NOT NULL,
  "reason_code" text NOT NULL DEFAULT '',
  "verification_method" text NULL,
  "verification_email" text NULL,
  "verified_at" timestamptz NULL,
  "verification_session_id" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prsrc_kind" CHECK (kind = ANY (ARRAY['WITHDRAW'::text, 'DECLINE'::text])),
  CONSTRAINT "chk_prsrc_verification_all_or_none" CHECK (((verification_method IS NULL) AND (verification_email IS NULL) AND (verified_at IS NULL) AND (verification_session_id IS NULL)) OR ((verification_method = 'GOOGLE'::text) AND (verification_email <> ''::text) AND (verified_at IS NOT NULL) AND (verification_session_id <> ''::text))),
  CONSTRAINT "chk_prsrc_withdrawn_version" CHECK ((kind = 'WITHDRAW'::text) = (withdrawn_version IS NOT NULL)),
  CONSTRAINT "chk_prsrc_withdrawn_version_positive" CHECK ((withdrawn_version IS NULL) OR (withdrawn_version >= 1))
);
-- Create index "uq_prsrc_idempotency" to table: "procurement_seller_response_commands"
CREATE UNIQUE INDEX "uq_prsrc_idempotency" ON "procurement_seller_response_commands" ("seller_response_id", "client_request_id");
