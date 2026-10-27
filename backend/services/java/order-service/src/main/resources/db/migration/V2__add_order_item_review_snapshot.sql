-- order_items 新增評論快照欄位（per order_item，凍結快照）
-- 來源：review-service review.created MQ 事件（見 03 設計書 §3.2 / §6.1 / §8.9）
-- 評論不可修改、不可刪除 → 快照寫入後永不變更，無 drift；NULL 表該品項未評論
ALTER TABLE order_items
    ADD COLUMN review JSONB;

COMMENT ON COLUMN order_items.review IS '該品項評論快照 {rating, content, imageUrls, createdAt}（NULL=未評論）。來源 review.created MQ 事件，評論不可改/刪 → 凍結快照';
