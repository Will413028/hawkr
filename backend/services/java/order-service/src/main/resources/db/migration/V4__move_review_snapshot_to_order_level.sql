-- 評論快照改為訂單層級（取該訂單最近一筆商品評論代表）
-- 前端訂單詳情「我的評價 / 商品評價」card 為訂單層級單一評論顯示，故快照存於 orders 而非 order_items。
-- 反轉 V2 加在 order_items 的 per-item 快照（破壞性：DROP COLUMN，僅移除本期 V2 新增、無正式資料）。
ALTER TABLE order_items DROP COLUMN IF EXISTS review;

ALTER TABLE orders ADD COLUMN review JSONB;

COMMENT ON COLUMN orders.review IS '訂單評論快照 {rating, content, imageUrls, createdAt}（NULL=未評論）。取該訂單最近一筆商品評論，來源 review.created MQ 事件，凍結快照';
