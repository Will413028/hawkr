-- 付款後出貨保證與逾期取消（見 03 設計書 §3.12 / §8.10 / §8.11）
-- 1. order.paid 可靠投遞 Outbox：付款成功事務內寫入，杜絕 afterCommit best-effort 發布遺失
-- 2. Backstop 重試追蹤欄位：付款後逾期無物流單 → 重試建單 → 超次取消退款

-- 1. 訂單事件發件匣（內部表，BIGSERIAL）
CREATE TABLE IF NOT EXISTS order_outbox (
    id            BIGSERIAL    PRIMARY KEY,
    aggregate_id  VARCHAR(64)  NOT NULL,
    event_type    VARCHAR(64)  NOT NULL,
    exchange      VARCHAR(128) NOT NULL,
    routing_key   VARCHAR(128) NOT NULL,
    payload       TEXT         NOT NULL,
    status        VARCHAR(16)  NOT NULL DEFAULT '8221',
    retry_count   INT          NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    sent_at       TIMESTAMPTZ
);

COMMENT ON TABLE order_outbox IS 'order.paid 等事件可靠投遞發件匣（與業務同事務寫入，OrderOutboxPublisherScheduler 輪詢發送）';
COMMENT ON COLUMN order_outbox.aggregate_id IS 'order_no';
COMMENT ON COLUMN order_outbox.status IS 'OutboxEventStatus value：8221=PENDING / 8222=SENT / 8223=FAILED / 8224=DEAD';

CREATE INDEX IF NOT EXISTS idx_order_outbox_dispatch
    ON order_outbox(status, next_retry_at)
    WHERE status <> '8222';

-- 2. Backstop 重試追蹤欄位
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment_create_attempts      SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment_create_next_retry_at TIMESTAMPTZ;

COMMENT ON COLUMN orders.shipment_create_attempts IS '付款後建單重試次數（Backstop，03 §8.11）';
COMMENT ON COLUMN orders.shipment_create_next_retry_at IS '下次建單重試時間（backoff）';

-- 3. Backstop 掃描部分索引（僅未建單的已付款訂單）
CREATE INDEX IF NOT EXISTS idx_orders_shipment_backstop
    ON orders(pay_time)
    WHERE status = '10' AND (shipment_no IS NULL OR shipment_no = '');
