-- Modify "procurement_requests" table
ALTER TABLE "procurement_requests" ADD COLUMN "resolved_crop_code" text NULL, ADD COLUMN "resolved_group_code" text NULL, ADD COLUMN "crop_disambiguation_options_json" jsonb NULL;
-- Create "procurement_quote_batches" table
CREATE TABLE "procurement_quote_batches" (
  "id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "resolved_crop_code" text NULL,
  "resolved_group_code" text NULL,
  "base_price_per_kg" bigint NOT NULL,
  "quantity" integer NOT NULL,
  "unit" text NOT NULL,
  "status" text NOT NULL DEFAULT 'OPEN',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_quote_batches_request" to table: "procurement_quote_batches"
CREATE INDEX "idx_procurement_quote_batches_request" ON "procurement_quote_batches" ("request_id", "created_at" DESC);
-- Create "procurement_quote_invites" table
CREATE TABLE "procurement_quote_invites" (
  "id" bigint NOT NULL,
  "quote_batch_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "candidate_id" bigint NOT NULL,
  "lane" text NOT NULL,
  "net_weight_kg" numeric(10,3) NULL,
  "markup_rate" numeric(5,4) NOT NULL,
  "seller_unit_price" bigint NOT NULL,
  "buyer_unit_price" bigint NOT NULL,
  "seller_total_price" bigint NULL,
  "buyer_total_price" bigint NULL,
  "channel" text NOT NULL,
  "confirm_token" text NOT NULL,
  "status" text NOT NULL DEFAULT 'PENDING',
  "confirmed_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_quote_invites_batch" to table: "procurement_quote_invites"
CREATE INDEX "idx_procurement_quote_invites_batch" ON "procurement_quote_invites" ("quote_batch_id");
-- Create index "idx_procurement_quote_invites_pending_by_request" to table: "procurement_quote_invites"
CREATE INDEX "idx_procurement_quote_invites_pending_by_request" ON "procurement_quote_invites" ("request_id") WHERE (status = 'PENDING'::text);
-- Create index "uq_procurement_quote_invites_token" to table: "procurement_quote_invites"
CREATE UNIQUE INDEX "uq_procurement_quote_invites_token" ON "procurement_quote_invites" ("confirm_token");
-- Create "procurement_quote_outbox" table
CREATE TABLE "procurement_quote_outbox" (
  "id" bigint NOT NULL,
  "event_type" text NOT NULL,
  "payload" jsonb NOT NULL,
  "status" text NOT NULL DEFAULT 'pending',
  "attempt_count" integer NOT NULL DEFAULT 0,
  "next_attempt_at" timestamptz NOT NULL DEFAULT now(),
  "last_error" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "completed_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_quote_outbox_pending" to table: "procurement_quote_outbox"
CREATE INDEX "idx_procurement_quote_outbox_pending" ON "procurement_quote_outbox" ("status", "next_attempt_at") WHERE (status = 'pending'::text);
