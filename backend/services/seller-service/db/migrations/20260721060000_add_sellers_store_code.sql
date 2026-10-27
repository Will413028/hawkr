-- Add store_code column to sellers table
ALTER TABLE "sellers" ADD COLUMN "store_code" bigint;

-- 對外顯示碼（YY+8 位亂數），建店當下 mint；nullable 是過渡態，Task 4 backfill 完才補 NOT NULL。
CREATE UNIQUE INDEX "uq_sellers_store_code" ON "sellers" ("store_code");
