-- Create "procurement_buyer_quote_acceptances" table
CREATE TABLE "procurement_buyer_quote_acceptances" (
  "id" bigint NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "publication_id" bigint NOT NULL,
  "seller_response_id" bigint NOT NULL,
  "seller_response_version" integer NOT NULL,
  "seller_response_payload_hash" text NOT NULL,
  "confirmed_rfq_snapshot_id" bigint NOT NULL,
  "confirmed_rfq_payload_hash" text NOT NULL,
  "lane" text NOT NULL,
  "store_id" bigint NULL,
  "prospect_id" bigint NULL,
  "accepted_deal_price" bigint NOT NULL,
  "currency" text NOT NULL,
  "goods_amount" bigint NOT NULL,
  "shipping_amount" bigint NOT NULL,
  "tax_amount" bigint NOT NULL,
  "state" text NOT NULL,
  "order_provisioning_state" text NOT NULL,
  "order_no" text NULL,
  "payment_deadline" timestamptz NULL,
  "client_request_id" text NOT NULL,
  "request_hash" text NOT NULL,
  "accepted_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prbqa_amounts_non_negative" CHECK ((goods_amount >= 0) AND (shipping_amount >= 0) AND (tax_amount >= 0)),
  CONSTRAINT "chk_prbqa_identity_matches_lane" CHECK (((lane = 'PLATFORM'::text) AND (store_id IS NOT NULL) AND (prospect_id IS NULL)) OR ((lane = 'EXTERNAL'::text) AND (prospect_id IS NOT NULL) AND (store_id IS NULL))),
  CONSTRAINT "chk_prbqa_order_no_matches_state" CHECK ((order_provisioning_state = 'ORDER_CREATED'::text) = (order_no IS NOT NULL)),
  CONSTRAINT "chk_prbqa_order_provisioning_state" CHECK (order_provisioning_state = ANY (ARRAY['CREATING_ORDER'::text, 'ORDER_CREATED'::text, 'ORDER_FAILED'::text])),
  CONSTRAINT "chk_prbqa_price_is_sum" CHECK (accepted_deal_price = ((goods_amount + shipping_amount) + tax_amount)),
  CONSTRAINT "chk_prbqa_state" CHECK (state = ANY (ARRAY['ACTIVE'::text, 'PAYMENT_FINALIZED'::text, 'CANCELLED'::text, 'EXPIRED'::text]))
);
-- Create index "idx_prbqa_seller_response" to table: "procurement_buyer_quote_acceptances"
CREATE INDEX "idx_prbqa_seller_response" ON "procurement_buyer_quote_acceptances" ("seller_response_id", "seller_response_version");
-- Create index "uq_prbqa_idempotency" to table: "procurement_buyer_quote_acceptances"
CREATE UNIQUE INDEX "uq_prbqa_idempotency" ON "procurement_buyer_quote_acceptances" ("buyer_id", "client_request_id");
-- Create index "uq_prbqa_order_no" to table: "procurement_buyer_quote_acceptances"
CREATE UNIQUE INDEX "uq_prbqa_order_no" ON "procurement_buyer_quote_acceptances" ("order_no") WHERE (order_no IS NOT NULL);
-- Create index "uq_prbqa_request_live" to table: "procurement_buyer_quote_acceptances"
CREATE UNIQUE INDEX "uq_prbqa_request_live" ON "procurement_buyer_quote_acceptances" ("request_id") WHERE (state = ANY (ARRAY['ACTIVE'::text, 'PAYMENT_FINALIZED'::text]));
-- Create "procurement_buyer_shortlist_entries" table
CREATE TABLE "procurement_buyer_shortlist_entries" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "seller_response_id" bigint NOT NULL,
  "seller_response_version" integer NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_prbse_request" to table: "procurement_buyer_shortlist_entries"
CREATE INDEX "idx_prbse_request" ON "procurement_buyer_shortlist_entries" ("request_id", "created_at");
-- Create index "uq_prbse_request_response" to table: "procurement_buyer_shortlist_entries"
CREATE UNIQUE INDEX "uq_prbse_request_response" ON "procurement_buyer_shortlist_entries" ("request_id", "seller_response_id");
-- Create "procurement_seller_response_drafts" table
CREATE TABLE "procurement_seller_response_drafts" (
  "seller_response_id" bigint NOT NULL,
  "payload" jsonb NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("seller_response_id")
);
-- Create "procurement_seller_response_versions" table
CREATE TABLE "procurement_seller_response_versions" (
  "id" bigint NOT NULL,
  "seller_response_id" bigint NOT NULL,
  "version" integer NOT NULL,
  "response_type" text NOT NULL,
  "offered_quality_grade" text NOT NULL,
  "offered_quality_inspection_notes" text NOT NULL,
  "unit_basis_quantity" numeric(14,3) NOT NULL,
  "unit_basis_unit" text NOT NULL,
  "offered_quantity" numeric(14,3) NOT NULL,
  "offered_quantity_unit" text NOT NULL,
  "conversion_basis" text NOT NULL,
  "currency" text NOT NULL,
  "goods_amount" bigint NOT NULL,
  "shipping_amount" bigint NOT NULL,
  "tax_amount" bigint NOT NULL,
  "tax_inclusion_basis" text NOT NULL,
  "total_amount" bigint NOT NULL,
  "lead_time_days" integer NULL,
  "promised_delivery_at" timestamptz NULL,
  "valid_until" timestamptz NOT NULL,
  "terms_note" text NOT NULL DEFAULT '',
  "deviation_disclosure" text NOT NULL DEFAULT '',
  "outside_buyer_requested_range" boolean NOT NULL,
  "payload_hash" text NOT NULL,
  "publication_version" integer NOT NULL,
  "seller_facing_payload_hash" text NOT NULL,
  "client_request_id" text NOT NULL,
  "submitted_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prsrv_amounts_non_negative" CHECK ((goods_amount >= 0) AND (shipping_amount >= 0) AND (tax_amount >= 0)),
  CONSTRAINT "chk_prsrv_quantities_positive" CHECK ((unit_basis_quantity > (0)::numeric) AND (offered_quantity > (0)::numeric)),
  CONSTRAINT "chk_prsrv_response_type" CHECK (response_type = ANY (ARRAY['MANUAL_QUOTE'::text, 'ACCEPT_NON_PRICE_TERMS_WITH_QUOTE'::text])),
  CONSTRAINT "chk_prsrv_tax_inclusion_basis" CHECK (tax_inclusion_basis = ANY (ARRAY['TAX_INCLUDED'::text, 'TAX_EXCLUDED'::text])),
  CONSTRAINT "chk_prsrv_total_is_sum" CHECK (total_amount = ((goods_amount + shipping_amount) + tax_amount)),
  CONSTRAINT "chk_prsrv_version_positive" CHECK (version >= 1)
);
-- Create index "uq_prsrv_idempotency" to table: "procurement_seller_response_versions"
CREATE UNIQUE INDEX "uq_prsrv_idempotency" ON "procurement_seller_response_versions" ("seller_response_id", "client_request_id");
-- Create index "uq_prsrv_response_version" to table: "procurement_seller_response_versions"
CREATE UNIQUE INDEX "uq_prsrv_response_version" ON "procurement_seller_response_versions" ("seller_response_id", "version");
-- Create "procurement_seller_responses" table
CREATE TABLE "procurement_seller_responses" (
  "id" bigint NOT NULL,
  "version" integer NOT NULL DEFAULT 1,
  "publication_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "request_id" bigint NOT NULL,
  "lane" text NOT NULL,
  "store_id" bigint NULL,
  "prospect_id" bigint NULL,
  "latest_version" integer NOT NULL DEFAULT 0,
  "withdrawn_version" integer NULL,
  "withdrawn_at" timestamptz NULL,
  "write_closed_at" timestamptz NULL,
  "write_closed_reason" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prsr_identity_matches_lane" CHECK (((lane = 'PLATFORM'::text) AND (store_id IS NOT NULL) AND (prospect_id IS NULL)) OR ((lane = 'EXTERNAL'::text) AND (prospect_id IS NOT NULL) AND (store_id IS NULL))),
  CONSTRAINT "chk_prsr_withdrawn_pair" CHECK ((withdrawn_version IS NULL) = (withdrawn_at IS NULL)),
  CONSTRAINT "chk_prsr_write_closed_pair" CHECK ((write_closed_at IS NULL) = (write_closed_reason IS NULL)),
  CONSTRAINT "chk_prsr_write_closed_reason" CHECK ((write_closed_reason IS NULL) OR (write_closed_reason = ANY (ARRAY['DECLINED'::text, 'PUBLICATION_CLOSED'::text, 'OTHER_WON'::text])))
);
-- Create index "idx_prsr_request_with_submission" to table: "procurement_seller_responses"
CREATE INDEX "idx_prsr_request_with_submission" ON "procurement_seller_responses" ("request_id", "buyer_id", "created_at" DESC) WHERE (latest_version > 0);
-- Create index "uq_prsr_publication_prospect" to table: "procurement_seller_responses"
CREATE UNIQUE INDEX "uq_prsr_publication_prospect" ON "procurement_seller_responses" ("publication_id", "prospect_id") WHERE (prospect_id IS NOT NULL);
-- Create index "uq_prsr_publication_store" to table: "procurement_seller_responses"
CREATE UNIQUE INDEX "uq_prsr_publication_store" ON "procurement_seller_responses" ("publication_id", "store_id") WHERE (store_id IS NOT NULL);
