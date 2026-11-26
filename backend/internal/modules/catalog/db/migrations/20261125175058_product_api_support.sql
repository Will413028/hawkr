-- Modify "products" table
ALTER TABLE "catalog"."products" ADD CONSTRAINT "products_low_stock_check" CHECK ((low_stock_threshold IS NULL) OR ((low_stock_threshold >= 0) AND (low_stock_threshold <= 9999))), ADD CONSTRAINT "products_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD CONSTRAINT "products_version_check" CHECK (content_version >= 1), ADD COLUMN "content_version" bigint NOT NULL DEFAULT 1, ADD COLUMN "low_stock_threshold" integer NULL, ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL;
-- Create index "products_request_key" to table: "products"
CREATE UNIQUE INDEX "products_request_key" ON "catalog"."products" ("store_id", "request_id") WHERE (request_id IS NOT NULL);
-- Modify "stock_operations" table
ALTER TABLE "catalog"."stock_operations" DROP CONSTRAINT "stock_operations_kind_check", ADD CONSTRAINT "stock_operations_kind_check" CHECK (kind = ANY (ARRAY['DEDUCT'::text, 'RESTORE'::text, 'ADJUST'::text, 'SET'::text])), DROP CONSTRAINT "stock_operations_store_check", ADD CONSTRAINT "stock_operations_store_check" CHECK ((kind = ANY (ARRAY['ADJUST'::text, 'SET'::text])) = (store_id IS NOT NULL)), ADD COLUMN "request_hash" bytea NULL;
-- 手寫：B4-2b 時期已存在的 ADJUST 操作沒有雜湊，下面的 CHECK 要求 ADJUST 與 SET 一定有。以操作唯一的 movement 重算應用層的雜湊
-- （idempotency.Hash("ADJUST", sku_id, delta) 的長度前綴編碼），同鍵重送的比對對舊操作仍然成立；沒有 movement 的（內容已不可知）以操作 id 的雜湊
-- 填入，任何重送都會被視為鍵重用。
UPDATE "catalog"."stock_operations" o
SET request_hash = sha256(convert_to('6:ADJUST|36:' || m.sku_id::text || '|' || length(m.delta::text) || ':' || m.delta::text || '|', 'UTF8'))
FROM "catalog"."stock_movements" m
WHERE m.operation_id = o.id AND o.kind = 'ADJUST' AND o.request_hash IS NULL;
UPDATE "catalog"."stock_operations" SET request_hash = sha256(convert_to(id::text, 'UTF8')) WHERE kind = 'ADJUST' AND request_hash IS NULL;
ALTER TABLE "catalog"."stock_operations" ADD CONSTRAINT "stock_operations_hash_check" CHECK ((kind = ANY (ARRAY['ADJUST'::text, 'SET'::text])) = (request_hash IS NOT NULL));
-- Create index "products_search_idx" to table: "products"
CREATE INDEX "products_search_idx" ON "catalog"."products" USING GIN ("searchable_text" public.gin_trgm_ops) WHERE (deleted_at IS NULL);
