-- ============================================================================
-- V11: 新增 sales_count_event_dedup 表（銷量累計 MQ 消費冪等保證）
-- 對應設計書：35-商品列表排序與銷量反正規化 §5
-- 背景：SalesAccumulatedConsumer 對 sales_count 純累加（sales_count = sales_count + qty），
--       MQ redelivery（MaxRetries requeue / 連線中斷重投 / 手動重放）會重複累加造成銷量虛增。
--       比照 favorite_count_event_dedup（V3），以事件自然鍵同事務 claim 保證 exactly-once。
-- 去重鍵：order_no（一張訂單的銷量只累計一次；涵蓋事件重投與重複發布兩種情境）。
-- ============================================================================

CREATE TABLE IF NOT EXISTS sales_count_event_dedup (
    order_no    VARCHAR(64)  NOT NULL,
    consumed_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (order_no)
);

CREATE INDEX IF NOT EXISTS idx_sales_dedup_consumed_at
    ON sales_count_event_dedup (consumed_at);

COMMENT ON TABLE  sales_count_event_dedup IS
    'order.sales.accumulated 事件去重表；SalesAccumulator.applyAll 同事務 INSERT ON CONFLICT DO NOTHING，rowsAffected=1 才逐筆累計 product.sales_count';
COMMENT ON COLUMN sales_count_event_dedup.order_no    IS '訂單編號（去重鍵；一張訂單的銷量只累計一次）';
COMMENT ON COLUMN sales_count_event_dedup.consumed_at IS '消費時間（DB DEFAULT NOW()）';
