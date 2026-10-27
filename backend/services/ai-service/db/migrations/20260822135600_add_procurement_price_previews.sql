-- Create "procurement_price_previews" table
CREATE TABLE "procurement_price_previews" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "request_revision_id" bigint NOT NULL,
  "pricing_mode" text NOT NULL,
  "quality_knowledge_state" text NOT NULL,
  "reference_availability" text NOT NULL,
  "reference_facts_hash" text NULL,
  "reference_payload_json" jsonb NULL,
  "unavailable_reason" text NULL,
  "buyer_requested_range_json" jsonb NOT NULL,
  "deal_temperature_json" jsonb NOT NULL,
  "client_request_id" text NOT NULL,
  "preview_input_hash" text NOT NULL,
  "payload_hash" text NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_procurement_price_previews_pricing_mode" CHECK (pricing_mode = ANY (ARRAY['PRICEBOOK_GUIDED'::text, 'BUYER_ENTERED_NO_REFERENCE'::text]))
);
-- Create index "uq_procurement_price_previews_idempotency" to table: "procurement_price_previews"
CREATE UNIQUE INDEX "uq_procurement_price_previews_idempotency" ON "procurement_price_previews" ("buyer_id", "client_request_id");
