-- Create "market_watch_target" table
CREATE TABLE "market_watch_target" (
  "id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "crop_id" bigint NOT NULL,
  "market_id" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "market_watch_target_crop_id_fkey" FOREIGN KEY ("crop_id") REFERENCES "crop" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "market_watch_target_market_id_fkey" FOREIGN KEY ("market_id") REFERENCES "market" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "idx_market_watch_target_account_created" to table: "market_watch_target"
CREATE INDEX "idx_market_watch_target_account_created" ON "market_watch_target" ("account_id", "created_at" DESC, "id" DESC);
-- Create index "uidx_market_watch_target_all_market" to table: "market_watch_target"
CREATE UNIQUE INDEX "uidx_market_watch_target_all_market" ON "market_watch_target" ("account_id", "crop_id") WHERE (market_id IS NULL);
-- Create index "uidx_market_watch_target_market" to table: "market_watch_target"
CREATE UNIQUE INDEX "uidx_market_watch_target_market" ON "market_watch_target" ("account_id", "crop_id", "market_id") WHERE (market_id IS NOT NULL);
