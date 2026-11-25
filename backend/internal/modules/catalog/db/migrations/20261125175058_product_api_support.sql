-- Modify "products" table
ALTER TABLE "catalog"."products" ADD CONSTRAINT "products_low_stock_check" CHECK ((low_stock_threshold IS NULL) OR ((low_stock_threshold >= 0) AND (low_stock_threshold <= 9999))), ADD CONSTRAINT "products_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD CONSTRAINT "products_version_check" CHECK (content_version >= 1), ADD COLUMN "content_version" bigint NOT NULL DEFAULT 1, ADD COLUMN "low_stock_threshold" integer NULL, ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL;
-- Create index "products_request_key" to table: "products"
CREATE UNIQUE INDEX "products_request_key" ON "catalog"."products" ("store_id", "request_id") WHERE (request_id IS NOT NULL);
-- Modify "stock_operations" table
ALTER TABLE "catalog"."stock_operations" DROP CONSTRAINT "stock_operations_kind_check", ADD CONSTRAINT "stock_operations_kind_check" CHECK (kind = ANY (ARRAY['DEDUCT'::text, 'RESTORE'::text, 'ADJUST'::text, 'SET'::text])), DROP CONSTRAINT "stock_operations_store_check", ADD CONSTRAINT "stock_operations_store_check" CHECK ((kind = ANY (ARRAY['ADJUST'::text, 'SET'::text])) = (store_id IS NOT NULL)), ADD CONSTRAINT "stock_operations_hash_check" CHECK ((kind = ANY (ARRAY['ADJUST'::text, 'SET'::text])) = (request_hash IS NOT NULL)), ADD COLUMN "request_hash" bytea NULL;
-- 手寫：Atlas 社群版不處理 CREATE EXTENSION（desired state 的 schema.sql 裡遇到就報錯），pg_trgm 與它的 GIN 索引由這裡建立。
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;
-- Create index "products_search_idx" to table: "products"
CREATE INDEX "products_search_idx" ON "catalog"."products" USING gin ("searchable_text" public.gin_trgm_ops) WHERE (deleted_at IS NULL);
