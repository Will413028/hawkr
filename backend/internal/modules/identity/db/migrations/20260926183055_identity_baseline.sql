-- Create "accounts" table
CREATE TABLE "identity"."accounts" (
  "id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "email" text NULL,
  "phone" text NULL,
  "suspended_at" timestamptz NULL,
  "deleted_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "accounts_live_check" CHECK (((deleted_at IS NULL) AND (phone IS NOT NULL)) OR ((deleted_at IS NOT NULL) AND (email IS NULL) AND (phone IS NULL) AND (suspended_at IS NULL))),
  CONSTRAINT "accounts_phone_check" CHECK (phone ~ '^09[0-9]{8}$'::text)
);
-- Create index "uidx_accounts_email" to table: "accounts"
CREATE UNIQUE INDEX "uidx_accounts_email" ON "identity"."accounts" ("email");
-- Create "apple_token_revocations" table
CREATE TABLE "identity"."apple_token_revocations" (
  "id" uuid NOT NULL,
  "client_id" text NOT NULL,
  "refresh_token" bytea NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create "phone_codes" table
CREATE TABLE "identity"."phone_codes" (
  "id" uuid NOT NULL,
  "phone" text NOT NULL,
  "code" text NOT NULL,
  "subject" text NOT NULL,
  "attempts" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "phone_codes_attempts_check" CHECK (attempts >= 0),
  CONSTRAINT "phone_codes_code_check" CHECK (code ~ '^[0-9]{6}$'::text),
  CONSTRAINT "phone_codes_phone_check" CHECK (phone ~ '^09[0-9]{8}$'::text)
);
-- Create index "idx_phone_codes_phone_created_at" to table: "phone_codes"
CREATE INDEX "idx_phone_codes_phone_created_at" ON "identity"."phone_codes" ("phone", "created_at");
-- Create index "idx_phone_codes_subject_created_at" to table: "phone_codes"
CREATE INDEX "idx_phone_codes_subject_created_at" ON "identity"."phone_codes" ("subject", "created_at");
-- Create "account_roles" table
CREATE TABLE "identity"."account_roles" (
  "account_id" bigint NOT NULL,
  "role" text NOT NULL,
  "source_id" bigint NULL,
  "granted_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("account_id", "role"),
  CONSTRAINT "account_roles_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "account_roles_role_check" CHECK (role = ANY (ARRAY['BUYER'::text, 'SELLER'::text]))
);
-- Create "link_tokens" table
CREATE TABLE "identity"."link_tokens" (
  "token_hash" bytea NOT NULL,
  "account_id" bigint NOT NULL,
  "provider" text NOT NULL,
  "subject" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz NOT NULL,
  "consumed_at" timestamptz NULL,
  PRIMARY KEY ("token_hash"),
  CONSTRAINT "link_tokens_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "link_tokens_provider_check" CHECK (provider = ANY (ARRAY['GOOGLE'::text, 'APPLE'::text]))
);
-- Create "oauth_links" table
CREATE TABLE "identity"."oauth_links" (
  "provider" text NOT NULL,
  "subject" text NOT NULL,
  "account_id" bigint NOT NULL,
  "apple_client_id" text NULL,
  "apple_refresh_token" bytea NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("provider", "subject"),
  CONSTRAINT "oauth_links_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "oauth_links_apple_token_check" CHECK (((apple_client_id IS NULL) AND (apple_refresh_token IS NULL)) OR ((provider = 'APPLE'::text) AND (apple_client_id IS NOT NULL) AND (apple_refresh_token IS NOT NULL))),
  CONSTRAINT "oauth_links_provider_check" CHECK (provider = ANY (ARRAY['GOOGLE'::text, 'APPLE'::text]))
);
-- Create index "idx_oauth_links_account_id" to table: "oauth_links"
CREATE INDEX "idx_oauth_links_account_id" ON "identity"."oauth_links" ("account_id");
-- Create "admins" table
CREATE TABLE "identity"."admins" (
  "id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "google_subject" text NOT NULL,
  "email" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "uidx_admins_google_subject" to table: "admins"
CREATE UNIQUE INDEX "uidx_admins_google_subject" ON "identity"."admins" ("google_subject");
-- Create "sessions" table
CREATE TABLE "identity"."sessions" (
  "token_hash" bytea NOT NULL,
  "account_id" bigint NULL,
  "admin_id" bigint NULL,
  "client" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "expires_at" timestamptz NOT NULL,
  "absolute_expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("token_hash"),
  CONSTRAINT "sessions_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "sessions_admin_id_fkey" FOREIGN KEY ("admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "sessions_client_check" CHECK (client = ANY (ARRAY['WEB'::text, 'APP'::text])),
  CONSTRAINT "sessions_expiry_check" CHECK (expires_at <= absolute_expires_at),
  CONSTRAINT "sessions_principal_check" CHECK (((account_id IS NOT NULL) AND (admin_id IS NULL)) OR ((account_id IS NULL) AND (admin_id IS NOT NULL) AND (client = 'WEB'::text)))
);
-- Create index "idx_sessions_account_id" to table: "sessions"
CREATE INDEX "idx_sessions_account_id" ON "identity"."sessions" ("account_id");
-- Create index "idx_sessions_admin_id" to table: "sessions"
CREATE INDEX "idx_sessions_admin_id" ON "identity"."sessions" ("admin_id");
