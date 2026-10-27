-- Create "crop_group" table
CREATE TABLE "crop_group" (
  "id" bigint NOT NULL,
  "group_code" text NOT NULL,
  "name" text NOT NULL,
  "category_code" text NULL,
  "active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "crop_group_group_code_key" UNIQUE ("group_code")
);
-- Create "ingest_run" table
CREATE TABLE "ingest_run" (
  "id" bigint NOT NULL,
  "kind" smallint NOT NULL,
  "range_start" date NOT NULL,
  "range_end" date NOT NULL,
  "status" smallint NOT NULL,
  "fetched_count" integer NOT NULL DEFAULT 0,
  "inserted_count" integer NOT NULL DEFAULT 0,
  "revised_count" integer NOT NULL DEFAULT 0,
  "error_message" text NULL,
  "started_at" timestamptz NOT NULL,
  "finished_at" timestamptz NULL,
  PRIMARY KEY ("id")
);
-- Create "crop" table
CREATE TABLE "crop" (
  "id" bigint NOT NULL,
  "crop_code" text NOT NULL,
  "group_id" bigint NOT NULL,
  "name" text NOT NULL,
  "variant_name" text NULL,
  "category_code" text NULL,
  "active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "crop_crop_code_key" UNIQUE ("crop_code"),
  CONSTRAINT "crop_group_id_fkey" FOREIGN KEY ("group_id") REFERENCES "crop_group" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "crop_group_id_idx" to table: "crop"
CREATE INDEX "crop_group_id_idx" ON "crop" ("group_id");
-- Create "market" table
CREATE TABLE "market" (
  "id" bigint NOT NULL,
  "market_code" text NOT NULL,
  "name" text NOT NULL,
  "county_code" text NULL,
  "latitude" numeric(9,6) NULL,
  "longitude" numeric(9,6) NULL,
  "active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "market_market_code_key" UNIQUE ("market_code")
);
-- Create "market_rest_day" table
CREATE TABLE "market_rest_day" (
  "id" bigint NOT NULL,
  "market_id" bigint NOT NULL,
  "rest_date" date NOT NULL,
  "reason" text NULL,
  "fetched_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "market_rest_day_market_id_rest_date_key" UNIQUE ("market_id", "rest_date"),
  CONSTRAINT "market_rest_day_market_id_fkey" FOREIGN KEY ("market_id") REFERENCES "market" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create "price_observation" table
CREATE TABLE "price_observation" (
  "id" bigint NOT NULL,
  "market_id" bigint NOT NULL,
  "crop_id" bigint NOT NULL,
  "trade_date" date NOT NULL,
  "revision_no" integer NOT NULL,
  "available_at" timestamptz NOT NULL,
  "avg_price" numeric(12,4) NULL,
  "upper_price" numeric(12,4) NULL,
  "middle_price" numeric(12,4) NULL,
  "lower_price" numeric(12,4) NULL,
  "volume_kg" numeric(14,3) NULL,
  "source" text NOT NULL,
  "source_digest" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "price_observation_market_id_crop_id_trade_date_revision_no_key" UNIQUE ("market_id", "crop_id", "trade_date", "revision_no"),
  CONSTRAINT "price_observation_crop_id_fkey" FOREIGN KEY ("crop_id") REFERENCES "crop" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "price_observation_market_id_fkey" FOREIGN KEY ("market_id") REFERENCES "market" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "price_observation_available_at_idx" to table: "price_observation"
CREATE INDEX "price_observation_available_at_idx" ON "price_observation" ("available_at");
-- Create index "price_observation_series_idx" to table: "price_observation"
CREATE INDEX "price_observation_series_idx" ON "price_observation" ("market_id", "crop_id", "trade_date" DESC);
