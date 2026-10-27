-- Modify "procurement_candidates" table
ALTER TABLE "procurement_candidates" ADD COLUMN "prospect_id" bigint NULL;
-- Create index "uq_procurement_candidates_set_prospect" to table: "procurement_candidates"
CREATE UNIQUE INDEX "uq_procurement_candidates_set_prospect" ON "procurement_candidates" ("candidate_set_id", "prospect_id") WHERE (prospect_id IS NOT NULL);
-- Create index "uq_procurement_candidates_set_store" to table: "procurement_candidates"
CREATE UNIQUE INDEX "uq_procurement_candidates_set_store" ON "procurement_candidates" ("candidate_set_id", "store_id") WHERE (store_id IS NOT NULL);
