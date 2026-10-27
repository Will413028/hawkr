-- Create "ingest_partition" table
CREATE TABLE "ingest_partition" (
  "market_id" bigint NOT NULL,
  "trade_date" date NOT NULL,
  "status" smallint NOT NULL,
  "attempt_count" integer NOT NULL DEFAULT 0,
  "declared_at" timestamptz NOT NULL DEFAULT now(),
  "last_attempt_at" timestamptz NULL,
  "completed_at" timestamptz NULL,
  "last_error" text NULL,
  PRIMARY KEY ("market_id", "trade_date"),
  CONSTRAINT "ingest_partition_market_id_fkey" FOREIGN KEY ("market_id") REFERENCES "market" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "ingest_partition_pending_idx" to table: "ingest_partition"
CREATE INDEX "ingest_partition_pending_idx" ON "ingest_partition" ("trade_date") WHERE (status = 1);
