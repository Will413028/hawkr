-- review-service schema
-- 訂單商品評論（一筆 = 買家對一個商品項目的評論）

CREATE TABLE order_item_reviews (
  id                BIGINT NOT NULL,
  order_no          VARCHAR(64) NOT NULL,
  order_item_id     BIGINT NOT NULL,
  buyer_id          BIGINT NOT NULL,
  store_id          BIGINT NOT NULL,
  product_id        BIGINT NOT NULL DEFAULT 0,
  rating            SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  content           TEXT,
  image_keys        TEXT[] NOT NULL DEFAULT '{}',   -- 評論附圖 objectKey 陣列（49 §5.3/§6.2；顯示 URL 由 Gateway 組）
  -- 商品資訊快照（避免評論頁需要回 order-service 查詢）
  sku_name          VARCHAR(255) NOT NULL DEFAULT '',
  item_image_key    TEXT NOT NULL DEFAULT '',       -- 商品圖 objectKey 快照（49 §5.3；來源=訂單 item 商品圖，顯示 URL 由 Gateway 組）
  price             BIGINT NOT NULL DEFAULT 0, -- 元，整數
  created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at        TIMESTAMP WITH TIME ZONE,
  hidden_at         TIMESTAMP WITH TIME ZONE,
  hidden_reason     TEXT,
  reply_content     TEXT,                      -- 賣家回覆內容（#1174，與 replied_at 同生同滅）
  replied_at        TIMESTAMP WITH TIME ZONE,  -- 賣家回覆時間（#1174，已回/未回唯一判斷源）
  CONSTRAINT order_item_reviews_pkey PRIMARY KEY (id),
  CONSTRAINT order_item_reviews_item_unique UNIQUE (order_item_id),
  -- reply_content 與 replied_at 必須同生同滅，杜絕「有時間沒內容」半套狀態（#1174）
  CONSTRAINT order_item_reviews_reply_consistency CHECK ((reply_content IS NULL) = (replied_at IS NULL))
);

CREATE INDEX idx_order_item_reviews_order_no ON order_item_reviews (order_no);
CREATE INDEX idx_order_item_reviews_buyer ON order_item_reviews (buyer_id, created_at DESC);
CREATE INDEX idx_order_item_reviews_seller ON order_item_reviews (store_id, created_at DESC);
CREATE INDEX idx_order_item_reviews_product ON order_item_reviews (product_id, created_at DESC);
