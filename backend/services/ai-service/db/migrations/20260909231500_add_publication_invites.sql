-- Create "procurement_rfq_publication_invites" table
CREATE TABLE "procurement_rfq_publication_invites" (
  "id" bigint NOT NULL,
  "publication_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "position" integer NOT NULL,
  "lane" text NOT NULL,
  "store_id" bigint NULL,
  "prospect_id" bigint NULL,
  "delivery_state" text NOT NULL,
  "delivery_state_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "chk_prpi_delivery_state" CHECK (delivery_state = ANY (ARRAY['NOT_SENT'::text, 'QUEUED'::text, 'SENT'::text, 'FAILED'::text])),
  CONSTRAINT "chk_prpi_identity_matches_lane" CHECK (((lane = 'PLATFORM'::text) AND (store_id IS NOT NULL) AND (prospect_id IS NULL)) OR ((lane = 'EXTERNAL'::text) AND (prospect_id IS NOT NULL) AND (store_id IS NULL)))
);
-- Create index "idx_prpi_publication" to table: "procurement_rfq_publication_invites"
CREATE INDEX "idx_prpi_publication" ON "procurement_rfq_publication_invites" ("publication_id");
-- Create index "uq_prpi_publication_position" to table: "procurement_rfq_publication_invites"
CREATE UNIQUE INDEX "uq_prpi_publication_position" ON "procurement_rfq_publication_invites" ("publication_id", "position");
-- Create index "uq_prpi_publication_prospect" to table: "procurement_rfq_publication_invites"
CREATE UNIQUE INDEX "uq_prpi_publication_prospect" ON "procurement_rfq_publication_invites" ("publication_id", "prospect_id") WHERE (prospect_id IS NOT NULL);
-- Create index "uq_prpi_publication_store" to table: "procurement_rfq_publication_invites"
CREATE UNIQUE INDEX "uq_prpi_publication_store" ON "procurement_rfq_publication_invites" ("publication_id", "store_id") WHERE (store_id IS NOT NULL);
