-- Modify "commodity" table
ALTER TABLE "commodity" DROP COLUMN "category_id";
-- Modify "category_crop_mapping" table
ALTER TABLE "category_crop_mapping" DROP CONSTRAINT "category_crop_mapping_resolved_oneof_check", ADD CONSTRAINT "category_crop_mapping_resolved_oneof_check" CHECK (((status = 'resolved'::text) AND ((group_code IS NULL) <> (commodity_id IS NULL))) OR ((status <> 'resolved'::text) AND (group_code IS NULL) AND (commodity_id IS NULL))), ADD COLUMN "commodity_id" bigint NULL, ADD CONSTRAINT "category_crop_mapping_commodity_id_fkey" FOREIGN KEY ("commodity_id") REFERENCES "commodity" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "category_crop_mapping_commodity_idx" to table: "category_crop_mapping"
CREATE INDEX "category_crop_mapping_commodity_idx" ON "category_crop_mapping" ("commodity_id") WHERE (commodity_id IS NOT NULL);
