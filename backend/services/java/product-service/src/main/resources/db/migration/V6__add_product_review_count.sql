-- ============================================================================
-- V6: 新增 product.review_count 反正規化欄位（評論數）
-- 對應設計書：02-產品微服務（Product Service）§3.2 / §5.14a（#1156）
-- 背景：商品列表/詳情 rating + reviewCount 聚合永遠為 0。product 端從未建置
--      評論聚合讀模型（無 review_count 欄位、rating 無 writer、無 review consumer）。
--      本欄位由 ReviewStatsConsumer 消費 review.created / review.hidden full-recompute 覆寫。
--      （rating 欄位 V1 已存在 decimal(3,2)，本次補上 writer，毋須新增欄位）
-- ============================================================================

ALTER TABLE product
    ADD COLUMN IF NOT EXISTS review_count BIGINT NOT NULL DEFAULT 0;

-- 防負值約束（與 favorite_count 對稱；full-recompute 覆寫理論上不會負，仍加保險）
ALTER TABLE product
    ADD CONSTRAINT chk_product_review_count_nonneg CHECK (review_count >= 0);

COMMENT ON COLUMN product.review_count IS
    '評論數反正規化（02 §5.14a，#1156）；ReviewStatsConsumer 消費 review.created / review.hidden 以 review-service 權威重算值 full-recompute 覆寫；業務寫入路徑禁止手動 set';
