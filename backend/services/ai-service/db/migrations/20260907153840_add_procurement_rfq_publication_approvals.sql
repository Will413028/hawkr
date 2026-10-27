-- Create "procurement_rfq_publication_approvals" table
CREATE TABLE "procurement_rfq_publication_approvals" (
  "id" bigint NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "request_revision_id" bigint NOT NULL,
  "confirmed_rfq_snapshot_id" bigint NOT NULL,
  "confirmed_rfq_payload_hash" text NOT NULL,
  "price_preview_id" bigint NOT NULL,
  "price_preview_version" bigint NOT NULL,
  "price_preview_payload_hash" text NOT NULL,
  "price_preview_input_hash" text NOT NULL,
  "quality_knowledge_state" text NOT NULL,
  "pricing_mode" text NOT NULL,
  "reference_availability" text NOT NULL,
  "buyer_requested_range_hash" text NOT NULL,
  "response_deadline" date NULL,
  "audience_mode" text NOT NULL,
  "audience_policy_id" text NOT NULL,
  "audience_policy_version" integer NOT NULL,
  "audience_hash" text NOT NULL,
  "candidate_set_id" bigint NULL,
  "candidate_set_policy_version" integer NULL,
  "seller_facing_payload_json" jsonb NOT NULL,
  "seller_facing_payload_hash" text NOT NULL,
  "deal_temperature_status" text NOT NULL DEFAULT 'UNKNOWN',
  "state" text NOT NULL,
  "schema_version" text NOT NULL,
  "preview_client_request_id" text NOT NULL,
  "generated_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prpa_direct_requires_candidate_set" CHECK ((audience_mode <> 'DIRECT_RECIPIENTS'::text) OR ((candidate_set_id IS NOT NULL) AND (candidate_set_policy_version IS NOT NULL))),
  CONSTRAINT "chk_prpa_wish_pool_has_no_candidate_set" CHECK ((audience_mode <> 'WISH_POOL'::text) OR ((candidate_set_id IS NULL) AND (candidate_set_policy_version IS NULL)))
);
-- Create index "idx_procurement_rfq_publication_approvals_request" to table: "procurement_rfq_publication_approvals"
CREATE INDEX "idx_procurement_rfq_publication_approvals_request" ON "procurement_rfq_publication_approvals" ("request_id", "buyer_id", "created_at" DESC);
-- Create index "uq_procurement_rfq_publication_approvals_idempotency" to table: "procurement_rfq_publication_approvals"
CREATE UNIQUE INDEX "uq_procurement_rfq_publication_approvals_idempotency" ON "procurement_rfq_publication_approvals" ("buyer_id", "preview_client_request_id");
