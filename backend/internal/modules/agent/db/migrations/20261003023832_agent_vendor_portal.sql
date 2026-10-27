-- Add new schema named "agent"
CREATE SCHEMA "agent";
-- Create "vendor_portal_invites" table
CREATE TABLE "agent"."vendor_portal_invites" (
  "invite_id" uuid NOT NULL,
  "rfq_recipient_id" bigint NOT NULL,
  "prospect_id" bigint NOT NULL,
  "invited_email" text NOT NULL,
  "secret_hash" bytea NOT NULL,
  "secret_delivered_at" timestamptz NULL,
  "expires_at" timestamptz NOT NULL,
  "revoked_at" timestamptz NULL,
  "revoke_reason" text NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("invite_id"),
  CONSTRAINT "vendor_portal_invites_rfq_recipient_key" UNIQUE ("rfq_recipient_id"),
  CONSTRAINT "vendor_portal_invites_secret_hash_key" UNIQUE ("secret_hash"),
  CONSTRAINT "vendor_portal_invites_email_check" CHECK ((invited_email <> ''::text) AND (invited_email = btrim(invited_email))),
  CONSTRAINT "vendor_portal_invites_prospect_check" CHECK (prospect_id > 0),
  CONSTRAINT "vendor_portal_invites_revoke_reason_check" CHECK (revoke_reason = 'PROSPECT_ARCHIVED'::text),
  CONSTRAINT "vendor_portal_invites_revoked_check" CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL)),
  CONSTRAINT "vendor_portal_invites_rfq_recipient_check" CHECK (rfq_recipient_id > 0),
  CONSTRAINT "vendor_portal_invites_secret_hash_check" CHECK (octet_length(secret_hash) = 32)
);
-- Create index "vendor_portal_invites_expires" to table: "vendor_portal_invites"
CREATE INDEX "vendor_portal_invites_expires" ON "agent"."vendor_portal_invites" ("expires_at");
-- Create index "vendor_portal_invites_prospect" to table: "vendor_portal_invites"
CREATE INDEX "vendor_portal_invites_prospect" ON "agent"."vendor_portal_invites" ("prospect_id");
-- Create index "vendor_portal_invites_revoked" to table: "vendor_portal_invites"
CREATE INDEX "vendor_portal_invites_revoked" ON "agent"."vendor_portal_invites" ("revoked_at") WHERE (revoked_at IS NOT NULL);
-- Create "vendor_portal_sessions" table
CREATE TABLE "agent"."vendor_portal_sessions" (
  "session_id" uuid NOT NULL,
  "token_hash" bytea NOT NULL,
  "invite_id" uuid NOT NULL,
  "verification_method" text NULL,
  "verified_email" text NULL,
  "verified_at" timestamptz NULL,
  "idle_expires_at" timestamptz NOT NULL,
  "absolute_expires_at" timestamptz NOT NULL,
  "revoked_at" timestamptz NULL,
  "last_seen_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("session_id"),
  CONSTRAINT "vendor_portal_sessions_token_hash_key" UNIQUE ("token_hash"),
  CONSTRAINT "vendor_portal_sessions_invite_id_fkey" FOREIGN KEY ("invite_id") REFERENCES "agent"."vendor_portal_invites" ("invite_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "vendor_portal_sessions_expiry_check" CHECK (idle_expires_at <= absolute_expires_at),
  CONSTRAINT "vendor_portal_sessions_method_check" CHECK (verification_method = 'GOOGLE'::text),
  CONSTRAINT "vendor_portal_sessions_token_hash_check" CHECK (octet_length(token_hash) = 32),
  CONSTRAINT "vendor_portal_sessions_verification_check" CHECK (((verification_method IS NULL) AND (verified_email IS NULL) AND (verified_at IS NULL)) OR ((verification_method IS NOT NULL) AND (verified_email IS NOT NULL) AND (verified_at IS NOT NULL)))
);
-- Create index "vendor_portal_sessions_absolute" to table: "vendor_portal_sessions"
CREATE INDEX "vendor_portal_sessions_absolute" ON "agent"."vendor_portal_sessions" ("absolute_expires_at");
-- Create index "vendor_portal_sessions_active" to table: "vendor_portal_sessions"
CREATE INDEX "vendor_portal_sessions_active" ON "agent"."vendor_portal_sessions" ("invite_id", "created_at" DESC) WHERE (revoked_at IS NULL);
