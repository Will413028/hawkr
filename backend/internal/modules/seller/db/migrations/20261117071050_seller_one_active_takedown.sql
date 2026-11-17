-- Create index "measures_id_status" to table: "measures"
CREATE UNIQUE INDEX "measures_id_status" ON "seller"."measures" ("id", "status");
-- Modify "measure_contents" table: add the column nullable, backfill from the owning measure, then constrain
ALTER TABLE "seller"."measure_contents" ADD COLUMN "measure_status" text NULL;
UPDATE "seller"."measure_contents" mc SET "measure_status" = m."status" FROM "seller"."measures" m WHERE m."id" = mc."measure_id";
ALTER TABLE "seller"."measure_contents" ALTER COLUMN "measure_status" SET NOT NULL, ALTER COLUMN "measure_status" SET DEFAULT 'ENFORCED';
ALTER TABLE "seller"."measure_contents" ADD CONSTRAINT "measure_contents_measure_status_fkey" FOREIGN KEY ("measure_id", "measure_status") REFERENCES "seller"."measures" ("id", "status") ON UPDATE CASCADE ON DELETE NO ACTION;
-- Create index "measure_contents_one_enforced" to table: "measure_contents"
CREATE UNIQUE INDEX "measure_contents_one_enforced" ON "seller"."measure_contents" ("content_kind", "content_id") WHERE (measure_status = 'ENFORCED'::text);
