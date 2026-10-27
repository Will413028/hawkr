-- Create "procurement_confirmed_rfq_snapshots" table
CREATE TABLE "procurement_confirmed_rfq_snapshots" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "request_revision_id" bigint NOT NULL,
  "price_preview_id" bigint NOT NULL,
  "price_preview_version" bigint NOT NULL,
  "quality_knowledge_state" text NOT NULL,
  "pricing_mode" text NOT NULL,
  "canonical_payload_json" jsonb NOT NULL,
  "payload_hash" text NOT NULL,
  "client_request_id" text NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "uq_procurement_confirmed_rfq_snapshots_idempotency" to table: "procurement_confirmed_rfq_snapshots"
CREATE UNIQUE INDEX "uq_procurement_confirmed_rfq_snapshots_idempotency" ON "procurement_confirmed_rfq_snapshots" ("buyer_id", "client_request_id");
