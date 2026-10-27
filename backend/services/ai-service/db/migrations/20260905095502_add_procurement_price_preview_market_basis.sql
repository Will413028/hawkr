-- Modify "procurement_price_previews" table
ALTER TABLE "procurement_price_previews" ADD COLUMN "market_price_basis_json" jsonb NULL, ADD COLUMN "quote_total_range_json" jsonb NULL;
