-- Create "procurement_usage" table
CREATE TABLE "procurement_usage" (
  "buyer_id" bigint NOT NULL,
  "period_start" timestamptz NOT NULL,
  "usage_count" integer NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("buyer_id", "period_start")
);
-- Create "procurement_usage_reservations" table
CREATE TABLE "procurement_usage_reservations" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "period_start" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_procurement_usage_reservations_buyer_period" to table: "procurement_usage_reservations"
CREATE INDEX "idx_procurement_usage_reservations_buyer_period" ON "procurement_usage_reservations" ("buyer_id", "period_start", "expires_at");
