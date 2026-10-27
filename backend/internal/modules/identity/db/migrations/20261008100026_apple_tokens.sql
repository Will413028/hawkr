-- Create "apple_tokens" table
CREATE TABLE "identity"."apple_tokens" (
  "account_id" bigint NOT NULL,
  "client_id" text NOT NULL,
  "token" bytea NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("account_id"),
  CONSTRAINT "apple_tokens_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "apple_tokens_client_id_check" CHECK (client_id <> ''::text)
);
