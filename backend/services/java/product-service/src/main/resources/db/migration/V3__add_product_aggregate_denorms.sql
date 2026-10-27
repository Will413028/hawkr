-- ============================================================================
-- V3: 新增 product 反正規化欄位（stock_qty / favorite_count / shipping_methods）
--     + favorite_count_event_dedup 表（收藏計數 MQ 消費冪等保證）
-- 對應設計書：02-產品微服務（Product Service）§3.2 / §3.5.1 / §5.13a / §5.14
-- ============================================================================

-- ----------------------------------------------------------------------------
-- 1. product 新增三個反正規化欄位
-- ----------------------------------------------------------------------------
ALTER TABLE product
    ADD COLUMN IF NOT EXISTS stock_qty        BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS favorite_count   BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS shipping_methods JSONB  NOT NULL DEFAULT '[]'::jsonb;

-- 防負值約束（防 buyer.favorite.removed 在 0 時 underflow）
ALTER TABLE product
    ADD CONSTRAINT chk_product_favorite_count_nonneg CHECK (favorite_count >= 0);

ALTER TABLE product
    ADD CONSTRAINT chk_product_stock_qty_nonneg CHECK (stock_qty >= 0);

-- 索引：收藏熱度排序（partial index，僅 ACTIVE + 未刪除商品；對應 02 §3.2）
CREATE INDEX IF NOT EXISTS idx_product_active_favorite_count
    ON product (favorite_count DESC, created_at DESC)
    WHERE deleted_at IS NULL AND status = '4004';

-- 索引：shipping_methods JSONB GIN（預留依配送方式聯集篩選用）
CREATE INDEX IF NOT EXISTS idx_product_shipping_methods_gin
    ON product USING GIN (shipping_methods jsonb_path_ops);


-- ----------------------------------------------------------------------------
-- 2. favorite_count_event_dedup 表：buyer.favorite event 冪等保證
--    對應設計書 §3.5.1
-- ----------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS favorite_count_event_dedup (
    event_id    UUID         NOT NULL,
    event_type  VARCHAR(40)  NOT NULL,           -- BuyerEventType value（BUYER_FAVORITE_ADDED / BUYER_FAVORITE_REMOVED）
    product_id  BIGINT       NOT NULL,
    consumed_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id)
);

CREATE INDEX IF NOT EXISTS idx_favorite_dedup_consumed_at
    ON favorite_count_event_dedup (consumed_at);

COMMENT ON TABLE  favorite_count_event_dedup IS
    'buyer.favorite.added/removed 事件去重表；FavoriteCountConsumer 同事務 INSERT ON CONFLICT DO NOTHING，rowsAffected=1 才執行 product.favorite_count ±1';
COMMENT ON COLUMN favorite_count_event_dedup.event_id   IS '由 buyer-service publisher 生成（UUID v4）';
COMMENT ON COLUMN favorite_count_event_dedup.event_type IS 'BuyerEventType.value（UPPER_SNAKE）';
COMMENT ON COLUMN favorite_count_event_dedup.consumed_at IS '消費時間；FavoriteEventDedupCleanupJob 每日 02:30 清理 30 天前';
