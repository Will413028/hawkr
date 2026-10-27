-- Modify "procurement_quote_batches" table
ALTER TABLE "procurement_quote_batches" ALTER COLUMN "base_price_per_kg" DROP NOT NULL, ADD COLUMN "price_availability" text NOT NULL DEFAULT 'UNAVAILABLE';
-- Backfill: 既有 row 在這欄新增前一律有非 null 的 base_price_per_kg（該欄當時仍是
-- NOT NULL），DEFAULT 'UNAVAILABLE' 只適用於「本 migration 之後才會出現」的真查無行情
-- row，不能讓它把這些歷史成交價 row 也標成查無行情。
UPDATE "procurement_quote_batches" SET "price_availability" = 'MARKET' WHERE "base_price_per_kg" IS NOT NULL;
-- Modify "procurement_quote_invites" table
ALTER TABLE "procurement_quote_invites" ALTER COLUMN "seller_unit_price" DROP NOT NULL, ALTER COLUMN "buyer_unit_price" DROP NOT NULL;
