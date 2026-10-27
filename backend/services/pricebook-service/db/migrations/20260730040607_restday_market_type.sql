-- Modify "market_rest_day" table
ALTER TABLE "market_rest_day" DROP CONSTRAINT "market_rest_day_market_id_rest_date_key", ADD COLUMN "market_type" text NULL, ADD CONSTRAINT "market_rest_day_market_id_market_type_rest_date_key" UNIQUE ("market_id", "market_type", "rest_date");
