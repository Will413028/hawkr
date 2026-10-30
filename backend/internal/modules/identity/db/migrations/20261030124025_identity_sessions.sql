-- Modify "accounts" table
ALTER TABLE "identity"."accounts" ALTER COLUMN "clerk_user_id" DROP NOT NULL;
-- Create "app_login_nonces" table
CREATE TABLE "identity"."app_login_nonces" (
  "nonce_hash" bytea NOT NULL,
  "created_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("nonce_hash"),
  CONSTRAINT "app_login_nonces_expires_check" CHECK (expires_at > created_at),
  CONSTRAINT "app_login_nonces_nonce_hash_check" CHECK (octet_length(nonce_hash) = 32)
);
-- Create index "idx_app_login_nonces_expires_at" to table: "app_login_nonces"
CREATE INDEX "idx_app_login_nonces_expires_at" ON "identity"."app_login_nonces" ("expires_at");
-- Create "account_identities" table
CREATE TABLE "identity"."account_identities" (
  "provider" text NOT NULL,
  "subject" text NOT NULL,
  "account_id" bigint NOT NULL,
  "email" text NULL,
  "email_verified" boolean NOT NULL DEFAULT false,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("provider", "subject"),
  CONSTRAINT "account_identities_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "account_identities_email_check" CHECK ((email IS NULL) OR (char_length(email) >= 1)),
  CONSTRAINT "account_identities_provider_check" CHECK (provider = ANY (ARRAY['GOOGLE'::text, 'LINE'::text])),
  CONSTRAINT "account_identities_subject_check" CHECK (char_length(subject) >= 1),
  CONSTRAINT "account_identities_verified_check" CHECK ((NOT email_verified) OR (email IS NOT NULL))
);
-- Create index "idx_account_identities_account_id" to table: "account_identities"
CREATE INDEX "idx_account_identities_account_id" ON "identity"."account_identities" ("account_id");
-- Create "login_flows" table
CREATE TABLE "identity"."login_flows" (
  "state_hash" bytea NOT NULL,
  "provider" text NOT NULL,
  "code_verifier" text NOT NULL,
  "nonce" text NOT NULL,
  "return_path" text NOT NULL,
  "binding_hash" bytea NOT NULL,
  "link_account_id" bigint NULL,
  "created_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("state_hash"),
  CONSTRAINT "login_flows_link_account_id_fkey" FOREIGN KEY ("link_account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "login_flows_binding_hash_check" CHECK (octet_length(binding_hash) = 32),
  CONSTRAINT "login_flows_expires_check" CHECK (expires_at > created_at),
  CONSTRAINT "login_flows_provider_check" CHECK (provider = ANY (ARRAY['GOOGLE'::text, 'LINE'::text])),
  CONSTRAINT "login_flows_return_path_check" CHECK ((return_path ~~ '/%'::text) AND (return_path !~~ '//%'::text) AND (return_path !~~ '/\\%'::text)),
  CONSTRAINT "login_flows_state_hash_check" CHECK (octet_length(state_hash) = 32)
);
-- Create index "idx_login_flows_expires_at" to table: "login_flows"
CREATE INDEX "idx_login_flows_expires_at" ON "identity"."login_flows" ("expires_at");
-- Create "sessions" table
CREATE TABLE "identity"."sessions" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "token_hash" bytea NOT NULL,
  "account_id" bigint NOT NULL,
  "client" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "last_used_at" timestamptz NOT NULL,
  "absolute_expires_at" timestamptz NOT NULL,
  "auth_time" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "sessions_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "sessions_client_check" CHECK (client = ANY (ARRAY['WEB'::text, 'APP'::text])),
  CONSTRAINT "sessions_times_check" CHECK ((last_used_at >= created_at) AND (absolute_expires_at > created_at) AND (auth_time >= created_at)),
  CONSTRAINT "sessions_token_hash_check" CHECK (octet_length(token_hash) = 32)
);
-- Create index "idx_sessions_account_id" to table: "sessions"
CREATE INDEX "idx_sessions_account_id" ON "identity"."sessions" ("account_id");
-- Create index "uidx_sessions_token_hash" to table: "sessions"
CREATE UNIQUE INDEX "uidx_sessions_token_hash" ON "identity"."sessions" ("token_hash");
