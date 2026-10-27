-- Create "category_price_rollup" table
CREATE TABLE "category_price_rollup" (
  "category_id" text NOT NULL,
  "level" smallint NOT NULL,
  "status" text NOT NULL,
  "latest_price" numeric(12,4) NULL,
  "prev_price" numeric(12,4) NULL,
  "points" jsonb NULL,
  "source_group_count" smallint NOT NULL,
  "computed_at" timestamptz NOT NULL,
  PRIMARY KEY ("category_id"),
  CONSTRAINT "category_price_rollup_level_check" CHECK (level = ANY (ARRAY[1, 2, 3])),
  CONSTRAINT "category_price_rollup_status_check" CHECK (status = ANY (ARRAY['ready'::text, 'unavailable'::text]))
);
-- Create index "category_price_rollup_level_idx" to table: "category_price_rollup"
CREATE INDEX "category_price_rollup_level_idx" ON "category_price_rollup" ("level");
