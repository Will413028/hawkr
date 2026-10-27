-- Create "procurement_rfq_publication_commands" table
CREATE TABLE "procurement_rfq_publication_commands" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "approval_id" bigint NOT NULL,
  "approval_version" integer NOT NULL,
  "client_request_id" text NOT NULL,
  "canonical_request_hash" text NOT NULL,
  "publication_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "uq_procurement_rfq_publication_commands_idempotency" to table: "procurement_rfq_publication_commands"
CREATE UNIQUE INDEX "uq_procurement_rfq_publication_commands_idempotency" ON "procurement_rfq_publication_commands" ("buyer_id", "client_request_id");
-- Create "procurement_rfq_publications" table
CREATE TABLE "procurement_rfq_publications" (
  "id" bigint NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "request_revision_id" bigint NOT NULL,
  "approval_id" bigint NOT NULL,
  "approval_version" integer NOT NULL,
  "command_id" bigint NOT NULL,
  "confirmed_rfq_snapshot_id" bigint NOT NULL,
  "seller_facing_payload_hash" text NOT NULL,
  "audience_mode" text NOT NULL,
  "audience_hash" text NOT NULL,
  "state" text NOT NULL,
  "schema_version" text NOT NULL,
  "published_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_rfq_publications_request" to table: "procurement_rfq_publications"
CREATE INDEX "idx_procurement_rfq_publications_request" ON "procurement_rfq_publications" ("request_id", "buyer_id", "created_at" DESC);
-- Create index "uq_procurement_rfq_publications_command" to table: "procurement_rfq_publications"
CREATE UNIQUE INDEX "uq_procurement_rfq_publications_command" ON "procurement_rfq_publications" ("command_id");
