-- Create "crop_group_forecast_result" table
CREATE TABLE "crop_group_forecast_result" (
  "id" bigint NOT NULL,
  "group_code" text NOT NULL,
  "generated_at" timestamptz NOT NULL,
  "target_date" date NOT NULL,
  "horizon_days" integer NOT NULL,
  "status" text NOT NULL,
  "missing_reason" text NULL,
  "aggregation_method" text NOT NULL,
  "effective_variant_count" integer NOT NULL,
  "total_variant_count" integer NOT NULL,
  "market_support_count" integer NOT NULL,
  "p10" numeric(12,4) NULL,
  "p50" numeric(12,4) NULL,
  "p90" numeric(12,4) NULL,
  "freshness" text NOT NULL,
  "model_version" text NOT NULL,
  "dataset_digest" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "crop_group_forecast_result_group_code_fkey" FOREIGN KEY ("group_code") REFERENCES "crop_group" ("group_code") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "crop_group_forecast_result_dedup_idx" to table: "crop_group_forecast_result"
CREATE UNIQUE INDEX "crop_group_forecast_result_dedup_idx" ON "crop_group_forecast_result" ("group_code", "target_date", (((generated_at AT TIME ZONE 'UTC'::text))::date));
-- Create index "crop_group_forecast_result_lookup_idx" to table: "crop_group_forecast_result"
CREATE INDEX "crop_group_forecast_result_lookup_idx" ON "crop_group_forecast_result" ("group_code", "target_date" DESC);
