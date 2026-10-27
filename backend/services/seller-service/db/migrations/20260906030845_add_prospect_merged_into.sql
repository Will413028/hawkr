-- Modify "seller_prospects" table
ALTER TABLE "seller_prospects" ADD CONSTRAINT "chk_sp_merged_into" CHECK ((merged_into_prospect_id IS NULL) OR (((status)::text = 'ARCHIVED'::text) AND (merged_into_prospect_id <> id))), ADD COLUMN "merged_into_prospect_id" bigint NULL, ADD CONSTRAINT "seller_prospects_merged_into_prospect_id_fkey" FOREIGN KEY ("merged_into_prospect_id") REFERENCES "seller_prospects" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "idx_sp_merged_into" to table: "seller_prospects"
CREATE INDEX "idx_sp_merged_into" ON "seller_prospects" ("merged_into_prospect_id") WHERE (merged_into_prospect_id IS NOT NULL);
