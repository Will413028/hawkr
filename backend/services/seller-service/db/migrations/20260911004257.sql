-- Create "vendor_portal_identities" table
CREATE TABLE "vendor_portal_identities" (
  "id" bigint NOT NULL,
  "prospect_id" bigint NOT NULL,
  "invited_email" character varying(255) NOT NULL,
  "status" character varying(20) NOT NULL DEFAULT 'ACTIVE',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "vendor_portal_identities_prospect_id_fkey" FOREIGN KEY ("prospect_id") REFERENCES "seller_prospects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_vpi_status" CHECK ((status)::text = ANY ((ARRAY['ACTIVE'::character varying, 'REVOKED'::character varying])::text[]))
);
-- Create index "uq_vpi_prospect" to table: "vendor_portal_identities"
CREATE UNIQUE INDEX "uq_vpi_prospect" ON "vendor_portal_identities" ("prospect_id");
-- Create "vendor_portal_invites" table
CREATE TABLE "vendor_portal_invites" (
  "id" bigint NOT NULL,
  "identity_id" bigint NOT NULL,
  "rfq_recipient_id" bigint NOT NULL,
  "token_hash" character varying(64) NOT NULL,
  "secret_delivered_at" timestamptz NULL,
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  "revoked_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "vendor_portal_invites_identity_id_fkey" FOREIGN KEY ("identity_id") REFERENCES "vendor_portal_identities" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "uq_vpinv_rfq_recipient" to table: "vendor_portal_invites"
CREATE UNIQUE INDEX "uq_vpinv_rfq_recipient" ON "vendor_portal_invites" ("rfq_recipient_id");
-- Create index "uq_vpinv_token_hash" to table: "vendor_portal_invites"
CREATE UNIQUE INDEX "uq_vpinv_token_hash" ON "vendor_portal_invites" ("token_hash");
-- Create "vendor_portal_sessions" table
CREATE TABLE "vendor_portal_sessions" (
  "id" bigint NOT NULL,
  "session_hash" character varying(64) NOT NULL,
  "identity_id" bigint NOT NULL,
  "invite_id" bigint NOT NULL,
  "assurance" character varying(20) NOT NULL,
  "idle_expires_at" timestamptz NOT NULL,
  "absolute_expires_at" timestamptz NOT NULL,
  "revoked_at" timestamptz NULL,
  "last_seen_at" timestamptz NOT NULL DEFAULT now(),
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "vendor_portal_sessions_identity_id_fkey" FOREIGN KEY ("identity_id") REFERENCES "vendor_portal_identities" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "vendor_portal_sessions_invite_id_fkey" FOREIGN KEY ("invite_id") REFERENCES "vendor_portal_invites" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_vps_assurance" CHECK ((assurance)::text = 'INVITE_ONLY'::text)
);
-- Create index "idx_vps_identity_active" to table: "vendor_portal_sessions"
CREATE INDEX "idx_vps_identity_active" ON "vendor_portal_sessions" ("identity_id") WHERE (revoked_at IS NULL);
-- Create index "uq_vps_session_hash" to table: "vendor_portal_sessions"
CREATE UNIQUE INDEX "uq_vps_session_hash" ON "vendor_portal_sessions" ("session_hash");
