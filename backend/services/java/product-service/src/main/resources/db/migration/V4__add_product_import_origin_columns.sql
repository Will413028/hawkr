-- ============================================================================
-- V4: product 表新增 import_job_id / import_item_id（02 號設計書 §9.4）
--
-- 用途：審計「正式商品從哪個 CSV 匯入轉正而來」
--   - 一般 CreateProduct：兩欄寫 NULL
--   - CreateProductFromImportInternal：兩欄帶 Import Service 傳入的 ID
-- 詳見：docs/詳細設計書/02-產品微服務（Product Service）詳細設計書.md §9.4
-- ============================================================================

ALTER TABLE product
    ADD COLUMN IF NOT EXISTS import_job_id  BIGINT,
    ADD COLUMN IF NOT EXISTS import_item_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_product_import_origin
    ON product (import_job_id, import_item_id)
    WHERE import_job_id IS NOT NULL;

COMMENT ON COLUMN product.import_job_id IS 'NULLABLE — 一般賣家上架為 NULL；經 Product Import Service 轉正才寫入（02 §9.4）';
COMMENT ON COLUMN product.import_item_id IS 'NULLABLE — 同上；用於審計「這個正式商品從哪 CSV row 來」';
