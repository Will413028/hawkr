-- Create "commodity" table
CREATE TABLE "commodity" (
  "id" bigint NOT NULL,
  "source" text NOT NULL,
  "commodity_code" text NOT NULL,
  "name" text NOT NULL,
  "price_layer" text NOT NULL,
  "unit" text NOT NULL,
  "category_id" text NULL,
  "active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "commodity_source_commodity_code_price_layer_key" UNIQUE ("source", "commodity_code", "price_layer"),
  CONSTRAINT "commodity_price_layer_check" CHECK (price_layer = ANY (ARRAY['wholesale_auction'::text, 'farmgate'::text, 'live_animal'::text, 'retail'::text, 'wholesale_bulk'::text]))
);
-- Create index "commodity_category_idx" to table: "commodity"
CREATE INDEX "commodity_category_idx" ON "commodity" ("category_id") WHERE (category_id IS NOT NULL);
-- Create index "commodity_source_idx" to table: "commodity"
CREATE INDEX "commodity_source_idx" ON "commodity" ("source");
-- Create "commodity_ingest_partition" table
CREATE TABLE "commodity_ingest_partition" (
  "source" text NOT NULL,
  "trade_date" date NOT NULL,
  "status" smallint NOT NULL,
  "attempt_count" integer NOT NULL DEFAULT 0,
  "declared_at" timestamptz NOT NULL DEFAULT now(),
  "last_attempt_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "last_error" text NULL,
  PRIMARY KEY ("source", "trade_date")
);
-- Create index "commodity_ingest_partition_status_idx" to table: "commodity_ingest_partition"
CREATE INDEX "commodity_ingest_partition_status_idx" ON "commodity_ingest_partition" ("status", "trade_date" DESC);
-- Create "commodity_observation" table
CREATE TABLE "commodity_observation" (
  "id" bigint NOT NULL,
  "commodity_id" bigint NOT NULL,
  "market_code" text NULL,
  "trade_date" date NOT NULL,
  "revision_no" integer NOT NULL,
  "available_at" timestamptz NOT NULL,
  "avg_price" numeric(12,4) NULL,
  "quantity" numeric(14,3) NULL,
  "quantity_unit" text NULL,
  "source" text NOT NULL,
  "source_digest" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "commodity_observation_commodity_id_fkey" FOREIGN KEY ("commodity_id") REFERENCES "commodity" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "commodity_observation_available_at_idx" to table: "commodity_observation"
CREATE INDEX "commodity_observation_available_at_idx" ON "commodity_observation" ("available_at");
-- Create index "commodity_observation_series_idx" to table: "commodity_observation"
CREATE INDEX "commodity_observation_series_idx" ON "commodity_observation" ("commodity_id", "trade_date" DESC);
-- Create index "uidx_commodity_observation_market" to table: "commodity_observation"
CREATE UNIQUE INDEX "uidx_commodity_observation_market" ON "commodity_observation" ("commodity_id", "market_code", "trade_date", "revision_no") WHERE (market_code IS NOT NULL);
-- Create index "uidx_commodity_observation_national" to table: "commodity_observation"
CREATE UNIQUE INDEX "uidx_commodity_observation_national" ON "commodity_observation" ("commodity_id", "trade_date", "revision_no") WHERE (market_code IS NULL);
