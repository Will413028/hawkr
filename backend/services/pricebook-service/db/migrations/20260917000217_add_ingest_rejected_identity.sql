CREATE TABLE "ingest_rejected_identity" (
  "source" text NOT NULL,
  "crop_code" text NOT NULL,
  "market_code" text NOT NULL,
  "reason" text NOT NULL,
  "occurrence_count" bigint NOT NULL DEFAULT 1,
  "first_seen_at" timestamptz NOT NULL DEFAULT now(),
  "last_seen_at" timestamptz NOT NULL DEFAULT now(),
  "first_trade_date" text NOT NULL,
  "last_trade_date" text NOT NULL,
  PRIMARY KEY ("source", "crop_code", "market_code", "reason")
);
CREATE INDEX "ingest_rejected_identity_last_seen_idx" ON "ingest_rejected_identity" ("last_seen_at" DESC);
