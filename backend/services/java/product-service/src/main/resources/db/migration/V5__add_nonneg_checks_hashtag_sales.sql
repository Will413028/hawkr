-- ============================================================================
-- V5: 反正規化計數欄位補非負 CHECK（防 underflow；issue #1141）
--     對齊既有 chk_product_favorite_count_nonneg / chk_product_stock_qty_nonneg（V3）模式。
--     hashtag.product_count：搭配 HashtagRepository.adjustProductCount 的下限守門（同 favorite_count）。
--     product.sales_count：DB 層防護（寫入路徑 SalesAccumulatedConsumer 已過濾 qty>0）。
-- ============================================================================

ALTER TABLE hashtag
    ADD CONSTRAINT chk_hashtag_product_count_nonneg CHECK (product_count >= 0);

ALTER TABLE product
    ADD CONSTRAINT chk_product_sales_count_nonneg CHECK (sales_count >= 0);
