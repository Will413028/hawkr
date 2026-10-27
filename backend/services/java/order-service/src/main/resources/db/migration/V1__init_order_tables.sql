-- =============================================
-- Order Service 資料庫初始化腳本
-- 11 張表：orders, order_items, refund_requests,
--          financial_logs, stock_tx_records,
--          payment_event_dedup, auto_refund_tasks,
--          refund_execute_tasks, coupon_consumption_tasks,
--          cart_cleanup_retry_tasks, grant_payment_tasks
-- =============================================

-- 建立 updated_at 自動更新觸發器函數
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 1. 訂單主表
CREATE TABLE orders (
    id                    BIGINT        PRIMARY KEY,
    order_no              VARCHAR(64)   NOT NULL,
    buyer_account_id      BIGINT        NOT NULL,
    seller_account_id     BIGINT        NOT NULL,
    total_original_amount BIGINT        NOT NULL,
    total_actual_amount   BIGINT        NOT NULL,
    status                VARCHAR(4)    NOT NULL DEFAULT '0',
    payment_method        VARCHAR(30),
    transaction_id        VARCHAR(128),
    checkout_id           VARCHAR(64),
    delivery_temp         VARCHAR(20),
    shipping_method       VARCHAR(30),
    shipping_fee          BIGINT        DEFAULT 0,
    shipping_fee_payer    VARCHAR(20),
    recipient_name        VARCHAR(50),
    recipient_phone       VARCHAR(20),
    shipping_address      VARCHAR(255),
    shipping_note         VARCHAR(255),
    order_remark          VARCHAR(500),                          -- 訂單備註（買家下單時填寫，選填）
    store_id              VARCHAR(20),
    store_name            VARCHAR(100),
    atm_bank_code         VARCHAR(10),                          -- ATM 銀行代碼（建單時快照，僅 ATM 付款）
    atm_virtual_account   VARCHAR(30),                          -- ATM 虛擬帳號（建單時快照，僅 ATM 付款）
    atm_expire_date       VARCHAR(30),                          -- ATM 繳費期限（建單時快照，僅 ATM 付款，ISO 8601）
    shipping_area         VARCHAR(20),
    sender_city           VARCHAR(20),                          -- 出貨城市（建單時從 Shipping Service 快照）
    sender_district       VARCHAR(20),                          -- 出貨地區（建單時從 Shipping Service 快照）
    dimension_sum         INTEGER,                              -- 材積總和（cm³，用於宅配 Spec 計算）
    mer_id                VARCHAR(30),
    coupon_instance_id    VARCHAR(64),                          -- 優惠券**實例** ID（reserveCoupon 後寫入；非 template id）
    discount_amount       BIGINT        DEFAULT 0,
    shipment_no           VARCHAR(64),
    shipping_status       VARCHAR(4) DEFAULT NULL,             -- 物流狀態 value（由 Shipping Service MQ 事件更新）
    shipping_status_desc  VARCHAR(200) DEFAULT NULL,          -- 物流狀態描述（由 Shipping Service MQ 事件快照寫入）
    label_url             VARCHAR(512) DEFAULT NULL,          -- 宅配託運單 PDF CDN URL（由 Shipping Service MQ 事件快照寫入）
    label_expire_at       TIMESTAMPTZ DEFAULT NULL,             -- 宅配託運單 PDF 下載期限（取號時 NOW()+24h，僅前端按鈕判斷用）
    partner_id            VARCHAR(10)  DEFAULT NULL,          -- 超商出貨便母代碼（3 碼，由 shipment.label.updated MQ 事件快照寫入）
    odno                  VARCHAR(20)  DEFAULT NULL,          -- 超商出貨單編號（8 碼，由 shipment.label.updated MQ 事件快照寫入）
    validation_no         VARCHAR(10)  DEFAULT NULL,          -- 超商 C2C 驗證碼（4 碼，由 shipment.label.updated MQ 事件快照寫入，B2C 時為空）
    shipment_photo_urls   JSONB        DEFAULT NULL,          -- 出貨照片 CDN URL 陣列（由 shipment.photo.updated MQ 事件快照寫入，JSON string array）
    receipt_photo_urls    JSONB        DEFAULT NULL,          -- 買家確認收貨上傳照片 CDN URL 陣列（ConfirmReceipt 寫入，JSON string array，選填）
    confirmed_at          TIMESTAMPTZ DEFAULT NULL,             -- 確認收貨時間（買家手動 / 自動確認時回填）
    confirm_type          VARCHAR(10) DEFAULT NULL,           -- 確認收貨方式：MANUAL（買家手動）/ AUTO（排程自動）
    payment_deadline      TIMESTAMPTZ,                          -- 付款期限（超時自動取消）
    payment_attempts      SMALLINT    NOT NULL DEFAULT 0,     -- 付款嘗試次數
    pay_time              TIMESTAMPTZ,
    ship_time             TIMESTAMPTZ,
    auto_complete_at      TIMESTAMPTZ,                          -- 自動完成時間（送達後 60 小時，遇 AfterSales case 活躍會延後，03 §5.8）
    countdown_paused_at   TIMESTAMPTZ DEFAULT NULL,             -- 倒數暫停時刻（NULL=倒數中；NOT NULL=已暫停，scheduler 跳過）；由 AfterSales MQ 事件觸發
    countdown_pause_count SMALLINT    NOT NULL DEFAULT 0,       -- 累計暫停次數（觀測用）
    processing_deadline   TIMESTAMPTZ DEFAULT NULL,             -- 賣家處理期限快照（UTC，09 §5.9）；shipment.created 寫入、shipment.processing.cleared 清空
    cancel_reason         VARCHAR(200),                       -- 取消原因 value（OrderCancelReason，91xx 系統 / 92xx 賣家；buyer 自願取消為 NULL）
    cancel_remark         VARCHAR(500),                       -- 取消備註（賣家自填，僅 OTHER / SUGGEST_SPLIT 場景使用）
    cancel_by_role        VARCHAR(20),                        -- 取消者角色（OrderCancelByRole：SYSTEM / SELLER / BUYER）
    delivered_at          TIMESTAMPTZ,                          -- 送達時間（訂單轉 DELIVERED 時記錄，供 AfterSales 時效驗證）
    need_refund_page      BOOLEAN     NOT NULL DEFAULT FALSE, -- ATM 退款等待買家前往 PayUni 頁面（由 MQ 事件驅動 TRUE/FALSE）
    is_reviewed           BOOLEAN     NOT NULL DEFAULT FALSE, -- 任一商品項目已評論即為 TRUE（由 review.created MQ 事件更新）
    created_at            TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uk_orders_order_no UNIQUE (order_no)
);

CREATE INDEX idx_orders_buyer_status ON orders (buyer_account_id, status);
CREATE INDEX idx_orders_seller_status ON orders (seller_account_id, status);
CREATE INDEX idx_orders_created_at ON orders (created_at);
CREATE INDEX idx_orders_transaction_id ON orders (transaction_id);
CREATE INDEX idx_orders_checkout_id ON orders (checkout_id) WHERE checkout_id IS NOT NULL;
CREATE INDEX idx_orders_payment_deadline ON orders (payment_deadline) WHERE status = '0' AND payment_deadline IS NOT NULL;
CREATE INDEX idx_orders_auto_complete ON orders (auto_complete_at) WHERE status = '40' AND auto_complete_at IS NOT NULL AND countdown_paused_at IS NULL;

DROP TRIGGER IF EXISTS update_orders_updated_at ON orders;
CREATE TRIGGER update_orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 2. 訂單明細表
CREATE TABLE order_items (
    id                BIGINT        PRIMARY KEY,
    order_no          VARCHAR(64)   NOT NULL,
    product_id        BIGINT,                                    -- 商品 ID 快照（下單時從 Product Service 複製）
    product_name      VARCHAR(255),                              -- 商品名稱快照（下單時從 Product Service 複製）
    sku_id            BIGINT        NOT NULL,
    sku_name          VARCHAR(255)  NOT NULL,
    quantity          INT           NOT NULL,
    list_price        BIGINT        NOT NULL,                   -- 原價快照（永遠 = 下單時 SKU.price，給前端 strike-through 顯示）
    unit_price        BIGINT        NOT NULL,                   -- 實際成交單價快照（無特價=list_price；有特價=當時 sale_price）。原欄位 original_price 重命名為本欄位
    share_discount    BIGINT        NOT NULL DEFAULT 0,
    actual_pay_amount BIGINT        NOT NULL,                   -- 實付金額（退款基準）= unit_price × quantity − share_discount
    refund_status     VARCHAR(4)    NOT NULL DEFAULT '0',
    image_object_key  VARCHAR(512),                         -- 商品圖片 objectKey 快照（下單時從 Product Service 複製）
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_order_items_order_sku UNIQUE (order_no, sku_id)
);

COMMENT ON COLUMN order_items.list_price IS '原價快照（新台幣元，永遠 = 下單時 SKU.price，與是否參與特價無關。給前端 strike-through 顯示用）';
COMMENT ON COLUMN order_items.unit_price IS '實際成交單價快照（新台幣元，無特價=list_price；有特價=當時 sale_price）。原欄位 original_price 已於本期重命名為本欄位';
COMMENT ON COLUMN order_items.share_discount IS '分攤的優惠券折扣（新台幣元，基於 unit_price × quantity 比例分攤）';
COMMENT ON COLUMN order_items.actual_pay_amount IS '實付金額（新台幣元，退款基準）= unit_price × quantity − share_discount';

CREATE INDEX idx_order_items_order_no ON order_items (order_no);
CREATE INDEX idx_order_items_sku_id ON order_items (sku_id);

-- 3. 退款申請表
CREATE TABLE refund_requests (
    request_no      VARCHAR(64)   PRIMARY KEY,
    order_no        VARCHAR(64)   NOT NULL,
    order_item_id   BIGINT        NOT NULL,
    reason          VARCHAR(255),
    status          VARCHAR(4)    NOT NULL DEFAULT '0',
    audit_admin_account_id BIGINT,
    audit_remark    VARCHAR(255),
    selected_method VARCHAR(4),
    refund_amount   BIGINT        NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_refund_requests_order_no ON refund_requests (order_no);
CREATE INDEX idx_refund_requests_status_time ON refund_requests (status, created_at);

DROP TRIGGER IF EXISTS update_refund_requests_updated_at ON refund_requests;
CREATE TRIGGER update_refund_requests_updated_at
    BEFORE UPDATE ON refund_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 4. 資金流水表（冪等：(type, external_txn_id) 唯一）
CREATE TABLE financial_logs (
    log_id          BIGINT        PRIMARY KEY,
    source_ref_no   VARCHAR(64)   NOT NULL,
    type            VARCHAR(4)    NOT NULL,
    amount          BIGINT        NOT NULL,
    external_txn_id VARCHAR(128)  NOT NULL,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_financial_logs_type_txn UNIQUE (type, external_txn_id)
);

CREATE INDEX idx_financial_logs_source ON financial_logs (source_ref_no);
CREATE INDEX idx_financial_logs_type_time ON financial_logs (type, created_at);
CREATE INDEX idx_financial_logs_external ON financial_logs (external_txn_id);

-- 5. 庫存交易協調表（複合主鍵：同一 txId 可分別記錄 DEDUCT 和 RESTORE）
CREATE TABLE stock_tx_records (
    stock_tx_id      VARCHAR(64)   NOT NULL,
    order_no         VARCHAR(64),
    phase            VARCHAR(16)   NOT NULL,
    status           VARCHAR(32)   NOT NULL,
    payload_snapshot JSONB         NOT NULL,
    last_error       TEXT,
    retry_count      INT           NOT NULL DEFAULT 0,
    next_retry_time  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (stock_tx_id, phase)
);

CREATE INDEX idx_stock_tx_order_no ON stock_tx_records (order_no);
CREATE INDEX idx_stock_tx_status_updated ON stock_tx_records (status, updated_at);
CREATE INDEX idx_stock_tx_next_retry ON stock_tx_records (next_retry_time);
CREATE INDEX IF NOT EXISTS idx_stock_tx_records_cleanup
    ON stock_tx_records (updated_at)
    WHERE status = 'SUCCESS';

DROP TRIGGER IF EXISTS update_stock_tx_records_updated_at ON stock_tx_records;
CREATE TRIGGER update_stock_tx_records_updated_at
    BEFORE UPDATE ON stock_tx_records
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 6. 支付事件冪等表
CREATE TABLE payment_event_dedup (
    transaction_id VARCHAR(128)  PRIMARY KEY,
    order_no       VARCHAR(64)   NOT NULL,
    event_type     VARCHAR(32)   NOT NULL,
    payload_hash   VARCHAR(64),
    processed_time TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- 7. 自動退款任務表
CREATE TABLE auto_refund_tasks (
    task_id         BIGINT        PRIMARY KEY,
    order_no        VARCHAR(64)   NOT NULL,
    transaction_id  VARCHAR(128)  NOT NULL,
    amount          BIGINT        NOT NULL,
    payment_method  VARCHAR(16),                              -- 付款方式（PaymentMethod dbValue，用於 Worker 判斷退款路徑）
    status          VARCHAR(32)   NOT NULL DEFAULT '0', -- WorkerTaskStatus value（0=PENDING / 5=AWAITING_BUYER_REFUND_PAGE / 10=PROCESSING / 15=AWAITING_WEBHOOK / 20=DONE / 30=FAILED / 40=MANUAL_INTERVENTION）
    last_error      TEXT,
    retry_count     INT           NOT NULL DEFAULT 0,
    max_retry_count INT           NOT NULL DEFAULT 10,
    next_retry_time TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uk_auto_refund_order_txn UNIQUE (order_no, transaction_id)
);

CREATE INDEX idx_auto_refund_status_retry ON auto_refund_tasks (status, next_retry_time);
CREATE INDEX idx_auto_refund_txn ON auto_refund_tasks (transaction_id);
CREATE INDEX IF NOT EXISTS idx_auto_refund_tasks_cleanup
    ON auto_refund_tasks (updated_at)
    WHERE status IN ('20', '40');

DROP TRIGGER IF EXISTS update_auto_refund_tasks_updated_at ON auto_refund_tasks;
CREATE TRIGGER update_auto_refund_tasks_updated_at
    BEFORE UPDATE ON auto_refund_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 8. 退款執行任務表
CREATE TABLE refund_execute_tasks (
    task_id         BIGINT        PRIMARY KEY,
    request_no      VARCHAR(64)   NOT NULL,
    order_no        VARCHAR(64)   NOT NULL,
    order_item_id   BIGINT        NOT NULL,
    method          VARCHAR(4)    NOT NULL,
    refund_amount   BIGINT        NOT NULL,
    status          VARCHAR(24)   NOT NULL DEFAULT '0',
    bank_code       VARCHAR(10),                            -- ATM 退款銀行代碼（3 碼）
    bank_sub_code   VARCHAR(10),                            -- ATM 退款分行代碼（4 碼）
    bank_account_no VARCHAR(128),                           -- ATM 退款銀行帳戶號碼（AES-256-GCM 加密存儲）
    uid             VARCHAR(128),                           -- ATM 退款身分證字號或統編（AES-256-GCM 加密存儲）
    external_ref_id VARCHAR(128),
    last_error      TEXT,
    retry_count     INT           NOT NULL DEFAULT 0,
    max_retry_count INT           NOT NULL DEFAULT 10,
    next_retry_time TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uk_refund_exec_request UNIQUE (request_no)
);

CREATE INDEX idx_refund_exec_status_retry ON refund_execute_tasks (status, next_retry_time);
CREATE INDEX idx_refund_exec_request ON refund_execute_tasks (request_no);
CREATE INDEX IF NOT EXISTS idx_refund_execute_tasks_cleanup
    ON refund_execute_tasks (updated_at)
    WHERE status IN ('DONE', 'MANUAL_INTERVENTION');

DROP TRIGGER IF EXISTS update_refund_execute_tasks_updated_at ON refund_execute_tasks;
CREATE TRIGGER update_refund_execute_tasks_updated_at
    BEFORE UPDATE ON refund_execute_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 9. 優惠券核銷任務表
CREATE TABLE coupon_consumption_tasks (
    task_id         BIGINT        PRIMARY KEY,
    order_no        VARCHAR(64)   NOT NULL,
    coupon_instance_id VARCHAR(64) NOT NULL,
    buyer_account_id BIGINT       NOT NULL,
    status          VARCHAR(24)   NOT NULL DEFAULT '0',
    last_error      TEXT,
    retry_count     INT           NOT NULL DEFAULT 0,
    max_retry_count INT           NOT NULL DEFAULT 10,
    next_retry_time TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uk_coupon_consumption_order_coupon UNIQUE (order_no, coupon_instance_id)
);

CREATE INDEX idx_coupon_consumption_status_retry ON coupon_consumption_tasks (status, next_retry_time);
CREATE INDEX IF NOT EXISTS idx_coupon_consumption_tasks_cleanup
    ON coupon_consumption_tasks (updated_at)
    WHERE status IN ('DONE', 'MANUAL_INTERVENTION');

DROP TRIGGER IF EXISTS update_coupon_consumption_tasks_updated_at ON coupon_consumption_tasks;
CREATE TRIGGER update_coupon_consumption_tasks_updated_at
    BEFORE UPDATE ON coupon_consumption_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 10. 購物車清理重試任務表
CREATE TABLE cart_cleanup_retry_tasks (
    task_id         BIGINT        PRIMARY KEY,
    order_no        VARCHAR(64)   NOT NULL,
    checkout_id     VARCHAR(64)   NOT NULL,
    buyer_account_id BIGINT       NOT NULL,
    status          VARCHAR(24)   NOT NULL DEFAULT '0',
    last_error      TEXT,
    retry_count     INT           NOT NULL DEFAULT 0,
    max_retry_count INT           NOT NULL DEFAULT 10,
    next_retry_time TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uk_cart_cleanup_order_checkout UNIQUE (order_no, checkout_id)
);

CREATE INDEX idx_cart_cleanup_status_retry ON cart_cleanup_retry_tasks (status, next_retry_time);
CREATE INDEX IF NOT EXISTS idx_cart_cleanup_tasks_cleanup
    ON cart_cleanup_retry_tasks (updated_at)
    WHERE status IN ('DONE', 'MANUAL_INTERVENTION');

DROP TRIGGER IF EXISTS update_cart_cleanup_retry_tasks_updated_at ON cart_cleanup_retry_tasks;
CREATE TRIGGER update_cart_cleanup_retry_tasks_updated_at
    BEFORE UPDATE ON cart_cleanup_retry_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 11. 撥款指示任務表（確認收貨後非同步撥款）
CREATE TABLE grant_payment_tasks (
    task_id         BIGINT        PRIMARY KEY,
    order_no        VARCHAR(64)   NOT NULL,
    confirm_type    VARCHAR(10)   NOT NULL,
    status          VARCHAR(24)   NOT NULL DEFAULT '0',
    last_error      TEXT,
    retry_count     INT           NOT NULL DEFAULT 0,
    max_retry_count INT           NOT NULL DEFAULT 10,
    next_retry_time TIMESTAMPTZ,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uk_grant_payment_order UNIQUE (order_no)
);

CREATE INDEX idx_grant_payment_status_retry ON grant_payment_tasks (status, next_retry_time);
-- 03 §3.11：status 一律存 WorkerTaskStatus value（'20'=DONE / '40'=MANUAL_INTERVENTION）
CREATE INDEX IF NOT EXISTS idx_grant_payment_tasks_cleanup
    ON grant_payment_tasks (updated_at)
    WHERE status IN ('20', '40');

DROP TRIGGER IF EXISTS update_grant_payment_tasks_updated_at ON grant_payment_tasks;
CREATE TRIGGER update_grant_payment_tasks_updated_at
    BEFORE UPDATE ON grant_payment_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
