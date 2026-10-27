-- Create "category_crop_mapping_crop" table
CREATE TABLE "category_crop_mapping_crop" (
  "category_id" text NOT NULL,
  "crop_id" bigint NOT NULL,
  PRIMARY KEY ("category_id", "crop_id"),
  CONSTRAINT "category_crop_mapping_crop_category_id_fkey" FOREIGN KEY ("category_id") REFERENCES "category_crop_mapping" ("category_id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "category_crop_mapping_crop_crop_id_fkey" FOREIGN KEY ("crop_id") REFERENCES "crop" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "category_crop_mapping_crop_crop_idx" to table: "category_crop_mapping_crop"
CREATE INDEX "category_crop_mapping_crop_crop_idx" ON "category_crop_mapping_crop" ("crop_id");
