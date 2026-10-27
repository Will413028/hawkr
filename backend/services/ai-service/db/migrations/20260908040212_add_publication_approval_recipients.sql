-- Create "procurement_rfq_publication_approval_recipients" table
CREATE TABLE "procurement_rfq_publication_approval_recipients" (
  "approval_id" bigint NOT NULL,
  "position" integer NOT NULL,
  "lane" text NOT NULL,
  "store_id" bigint NULL,
  "prospect_id" bigint NULL,
  PRIMARY KEY ("approval_id", "position"),
  CONSTRAINT "chk_prpar_identity_matches_lane" CHECK (((lane = 'PLATFORM'::text) AND (store_id IS NOT NULL) AND (prospect_id IS NULL)) OR ((lane = 'EXTERNAL'::text) AND (prospect_id IS NOT NULL) AND (store_id IS NULL)))
);
