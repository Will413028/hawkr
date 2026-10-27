-- Drop index "idx_sp_merged_into" from table: "seller_prospects"
DROP INDEX "idx_sp_merged_into";
-- Drop index "uq_seller_prospects_tax_id" from table: "seller_prospects"
DROP INDEX "uq_seller_prospects_tax_id";
-- Create index "uq_seller_prospects_tax_id" to table: "seller_prospects"
CREATE UNIQUE INDEX "uq_seller_prospects_tax_id" ON "seller_prospects" ("tax_id") WHERE ((tax_id IS NOT NULL) AND ((status)::text <> 'ARCHIVED'::text));
-- Create index "idx_seller_prospects_merged_into" to table: "seller_prospects"
CREATE INDEX "idx_seller_prospects_merged_into" ON "seller_prospects" ("merged_into_prospect_id") WHERE (merged_into_prospect_id IS NOT NULL);
