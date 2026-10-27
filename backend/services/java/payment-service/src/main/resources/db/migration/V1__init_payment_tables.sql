-- =====================================================
-- Payment Service 資料庫初始化腳本
-- 建立支付服務所需的所有資料表
-- =====================================================

-- -----------------------------------------------------
-- 1. 支付交易主表 (payment_transactions)
-- -----------------------------------------------------
CREATE TABLE payment_transactions (
    id              BIGINT PRIMARY KEY,                 -- TSID (時序 ID)
    trade_no        VARCHAR(32) NOT NULL UNIQUE,        -- 系統交易編號 = {orderNo}{3位序號}（如 O382716459001），即 PayUni MerTradeNo。每次付款嘗試獨立 trade_no
    order_id        VARCHAR(64) NOT NULL,               -- 原始訂單編號（如 O382716459），同一訂單的所有 row 共用
    attempt_seq     SMALLINT NOT NULL DEFAULT 1,        -- 同一訂單的第 N 次付款嘗試（從 1 開始遞增）
    mer_id     VARCHAR(32),                        -- 商家編號（選填）
    account_id      VARCHAR(64),                        -- 帳號編號

    -- 交易資訊
    payment_type    VARCHAR(4) NOT NULL,                -- 支付方式 value (2001:信用卡, 2002:ApplePay, 2003:GooglePay, 2004:ATM, 2900:ZERO_AMOUNT)
    amount          BIGINT NOT NULL,                    -- 交易金額（新台幣元）
    product_name    VARCHAR(200),                       -- 商品名稱
    status          VARCHAR(4) NOT NULL DEFAULT '0',    -- 交易狀態 value (0:INIT, 1:PAID, 2:FAILED, 3:EXPIRED, 4:CONFLICT, 5:CLOSED, 6:CLOSED_BY_SWITCH)

    -- PayUni 回傳資訊（遵循 docs/開發規範/PayUni 錯誤回應收斂規範.md §4.1 SoT）
    provider_trade_no   VARCHAR(64),                    -- PayUni 交易編號
    provider_status     VARCHAR(20),                    -- PayUni TradeStatus 原始值
    provider_message    TEXT,                           -- PayUni 回傳訊息

    -- 撥款（Grant）欄位（僅 Admin API 暴露，見規範 §6.2 封閉規則）
    granted_at          TIMESTAMPTZ,                    -- PayUni 撥款成功時間（里程碑）
    grant_status        VARCHAR(20),                    -- PayUni /api/merchant/grant 回應狀態碼
    grant_message       TEXT,                           -- PayUni 撥款回應訊息

    -- ATM 專用
    atm_bank_code       VARCHAR(10),                    -- 銀行代碼
    atm_virtual_account VARCHAR(20),                    -- 虛擬帳戶
    atm_expire_date     DATE,                           -- 繳費期限

    -- 時間戳記
    pay_time        TIMESTAMPTZ,           -- 付款完成時間
    closed_at       TIMESTAMPTZ,           -- 關閉時間
    expire_at       TIMESTAMPTZ,           -- 逾期時間
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- L2 補救路徑（PendingPaymentReconcileScheduler）
    last_reconciled_at TIMESTAMPTZ,        -- 最近一次 reconcile 處理時間，控制重撈退避
    rescued_at         TIMESTAMPTZ,        -- rescue 退款觸發時間（EXPIRED→PayUni SUCCESS 衝突時設定）
    rescue_action      VARCHAR(30),        -- PaymentRescueAction value（8901=REFUNDED / 8902=REFUND_PENDING_MANUAL）

    -- §2A.6 重複付款處理欄位
    is_duplicate              BOOLEAN NOT NULL DEFAULT FALSE,  -- 是否為重複付款 row
    duplicate_of_payment_id   BIGINT NULL,                     -- 指向「最早成功 row」的 id
    duplicate_sequence        SMALLINT NULL,                   -- 同訂單第 N 次付款（1=首筆，2,3...為重複）

    -- §10.5 PayUni 真實時間軸
    provider_create_at        TIMESTAMPTZ NULL,                -- PayUni 認為的交易建立時間
    provider_paid_at          TIMESTAMPTZ NULL,                -- PayUni 認為使用者真實付款時間

    -- §2A.5.4 ATM 復活機制
    reactivated_at            TIMESTAMPTZ NULL,                -- ATM row 最後一次復活時間（審計用）

    -- §5.9 請求端類型 / App 身份（CodeMeta ClientType / AppType；決定 PayUni 回調最終出口）
    client_type     VARCHAR(16) NOT NULL DEFAULT 'MOBILE_APP', -- ClientType value（MOBILE_APP / WEB_BROWSER）
    app_type        VARCHAR(8)  NOT NULL DEFAULT 'BUYER',      -- AppType value（BUYER / SELLER）

    -- 版本控制（樂觀鎖）
    version         INTEGER NOT NULL DEFAULT 0,

    -- 約束
    CONSTRAINT chk_payment_type CHECK (payment_type IN ('2001', '2002', '2003', '2004', '2900')),
    CONSTRAINT chk_status CHECK (status IN ('0', '1', '2', '3', '4', '5', '6')),
    -- §2A.1：ZERO_AMOUNT(2900) 必為 0、其他付款方式必 > 0，雙向不變式由 DB 端硬保護
    CONSTRAINT chk_amount_valid CHECK (
        (payment_type = '2900' AND amount = 0)
        OR (payment_type <> '2900' AND amount > 0)
    ),
    CONSTRAINT chk_rescue_action CHECK (rescue_action IS NULL OR rescue_action IN ('8901', '8902'))
);

-- 索引
CREATE INDEX idx_payment_transactions_order_id ON payment_transactions(order_id);
CREATE INDEX idx_payment_transactions_account_id ON payment_transactions(account_id);
CREATE INDEX idx_payment_transactions_status ON payment_transactions(status);
CREATE INDEX idx_payment_transactions_created_at ON payment_transactions(created_at);
-- §2A.3.6 provider_trade_no 唯一（PayUni 不會重複），用 partial index 允許 NULL 共存
CREATE UNIQUE INDEX uq_payment_transactions_provider_trade_no
    ON payment_transactions(provider_trade_no)
    WHERE provider_trade_no IS NOT NULL;
CREATE INDEX idx_payment_transactions_expire_at ON payment_transactions(expire_at) WHERE status = '0';
-- §2A.6 重複付款查詢索引
CREATE INDEX idx_payment_transactions_duplicate
    ON payment_transactions(duplicate_of_payment_id)
    WHERE is_duplicate = TRUE;
-- 同一訂單同時只能有一筆活躍交易（INIT 或 PAID 且非 duplicate），防止重複扣款
-- §2A.6 段 1.b reactivate 路徑：標 is_duplicate=TRUE 的副 row 不佔 index slot
CREATE UNIQUE INDEX idx_payment_transactions_order_active
    ON payment_transactions(order_id)
    WHERE status IN ('0', '1') AND is_duplicate = FALSE;

-- L2 補救路徑掃描專用部分索引：涵蓋 INIT 與 grace 視窗內的 EXPIRED
CREATE INDEX idx_payment_transactions_reconcile
    ON payment_transactions (created_at, last_reconciled_at)
    WHERE status IN ('0', '3');

COMMENT ON TABLE payment_transactions IS '支付交易主表';
COMMENT ON COLUMN payment_transactions.trade_no IS '系統交易編號 = {orderNo}{3位序號}（如 O382716459001），即 PayUni MerTradeNo';
COMMENT ON COLUMN payment_transactions.attempt_seq IS '同一訂單的第 N 次付款嘗試（從 1 開始遞增）';
COMMENT ON COLUMN payment_transactions.status IS '交易狀態：0=INIT, 1=PAID, 2=FAILED, 3=EXPIRED, 4=CONFLICT, 5=CLOSED, 6=CLOSED_BY_SWITCH';
COMMENT ON COLUMN payment_transactions.last_reconciled_at IS '最近一次 PendingPaymentReconcileScheduler 處理時間，用於控制重撈退避';
COMMENT ON COLUMN payment_transactions.rescued_at IS 'rescue 退款觸發時間，僅 EXPIRED→PayUni SUCCESS 衝突時設定';
COMMENT ON COLUMN payment_transactions.rescue_action IS 'rescue 動作結果：REFUNDED（信用卡同步退款完成）/ REFUND_PENDING_MANUAL（ATM 待人工頁面版完成）';
COMMENT ON INDEX idx_payment_transactions_order_active IS '同一訂單同時只能有一筆活躍交易（INIT 或 PAID），防止重複扣款';
COMMENT ON INDEX idx_payment_transactions_reconcile IS 'L2 補救路徑掃描專用，涵蓋 INIT 與 grace 視窗內的 EXPIRED';

-- -----------------------------------------------------
-- 2. 退款記錄表 (payment_refunds)
-- -----------------------------------------------------
CREATE TABLE payment_refunds (
    id              BIGINT PRIMARY KEY,                 -- TSID (時序 ID)
    refund_no       VARCHAR(32) NOT NULL UNIQUE,        -- 系統退款編號 (RFD{yyyyMMdd}{HHmmss}{random4})
    payment_id      BIGINT NOT NULL,                    -- §2A.3.6 對應 payment_transactions(id)，FK 改指 PK
    trade_no        VARCHAR(32) NOT NULL,               -- 原交易編號（保留供查詢/顯示，不再 UNIQUE）
    order_id        VARCHAR(64) NOT NULL,               -- 訂單編號

    -- 退款資訊
    amount          BIGINT NOT NULL,                    -- 退款金額（新台幣元）
    status          VARCHAR(4) NOT NULL DEFAULT '0',    -- 退款狀態 value (0:REQUESTED, 1:PROCESSING, 2:SUCCEEDED, 3:FAILED)
    reason          VARCHAR(500),                       -- 退款原因

    -- 冪等鍵（AfterSales 傳入 case_no 或 cancel_id，防止重複退款）
    idempotency_key VARCHAR(64),

    -- PayUni 回傳資訊（遵循 docs/開發規範/PayUni 錯誤回應收斂規範.md §4.1 SoT + §4.4 單一欄位雙階段寫入）
    provider_refund_no  VARCHAR(64),                    -- PayUni 退款編號
    provider_status     VARCHAR(20),                    -- PayUni 退款狀態（sync: response.Status；async webhook: inner RefundStatus 1/2）
    provider_message    TEXT,                           -- PayUni 回傳訊息

    -- 時間戳記
    completed_at    TIMESTAMPTZ,           -- 退款完成時間
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- ATM 退款 PayUni 里程碑日期（§3.4 規範允許的里程碑時間戳）
    estimated_refund_date DATE NULL,        -- ATM sync success 時 PayUni 回報的預計扣款日（YYYY-MM-DD）
    bank_date             DATE NULL,        -- ATM async webhook RefundStatus=1 時的實際到帳日（YYYY-MM-DD）

    -- 版本控制
    version         INTEGER NOT NULL DEFAULT 0,

    -- 約束
    CONSTRAINT chk_refund_status CHECK (status IN ('0', '1', '2', '3')),
    CONSTRAINT chk_refund_amount_positive CHECK (amount > 0),

    -- §2A.3.6 FK 改指向 payment_transactions(id)（PK），不再依賴 trade_no 唯一性
    CONSTRAINT fk_refund_payment FOREIGN KEY (payment_id)
        REFERENCES payment_transactions(id)
        ON DELETE RESTRICT
);

CREATE INDEX idx_payment_refunds_payment_id ON payment_refunds(payment_id);
CREATE INDEX idx_payment_refunds_trade_no ON payment_refunds(trade_no);
CREATE INDEX idx_payment_refunds_order_id ON payment_refunds(order_id);
CREATE INDEX idx_payment_refunds_status ON payment_refunds(status);
CREATE INDEX idx_payment_refunds_created_at ON payment_refunds(created_at);
CREATE UNIQUE INDEX idx_payment_refunds_idempotency_key ON payment_refunds(idempotency_key) WHERE idempotency_key IS NOT NULL;

COMMENT ON TABLE payment_refunds IS '退款記錄表';
COMMENT ON COLUMN payment_refunds.status IS '退款狀態：0=REQUESTED, 1=PROCESSING, 2=SUCCEEDED, 3=FAILED';
COMMENT ON COLUMN payment_refunds.idempotency_key IS '冪等鍵（AfterSales case_no 或 cancel_id），防止重複退款';

-- -----------------------------------------------------
-- 3. Webhook 記錄表 (payment_webhooks)
-- 支援多種 NotifyType（付款通知、商家狀態異動等）
-- -----------------------------------------------------
CREATE TABLE payment_webhooks (
    id              BIGSERIAL PRIMARY KEY,              -- 自增主鍵（內部使用）
    webhook_type    VARCHAR(50) NOT NULL DEFAULT '8241', -- WebhookProvider.PAYUNI
    trade_no        VARCHAR(32),                        -- 對應系統交易編號（付款 Webhook 使用）
    provider_trade_no VARCHAR(64),                      -- PayUni 交易編號（付款 Webhook 使用）
    idempotency_key VARCHAR(128),                       -- 冪等鍵（依 webhook_type 組合不同欄位）

    -- 原始資料
    raw_request     TEXT NOT NULL,                      -- 原始請求內容
    decrypted_payload JSONB,                            -- 解密後資料

    -- 處理狀態
    is_valid        BOOLEAN NOT NULL DEFAULT FALSE,     -- 驗證是否通過
    is_processed    BOOLEAN NOT NULL DEFAULT FALSE,     -- 是否已處理
    process_status  VARCHAR(20) NOT NULL DEFAULT '8201', -- 處理狀態 WebhookProcessStatus value（8201=PENDING / 8202=SUCCESS / 8203=FAILED / 8204=SKIPPED / 8205=VERIFY_FAILED / 8206=DECRYPT_FAILED）
    error_message   TEXT,                               -- 錯誤訊息

    -- 時間戳記
    received_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at    TIMESTAMPTZ,

    -- 來源資訊
    source_ip       VARCHAR(45),                        -- 來源 IP
    user_agent      VARCHAR(500)                        -- User-Agent
);

CREATE INDEX idx_payment_webhooks_webhook_type ON payment_webhooks(webhook_type);
CREATE INDEX idx_payment_webhooks_trade_no ON payment_webhooks(trade_no);
CREATE INDEX idx_payment_webhooks_received_at ON payment_webhooks(received_at);
CREATE INDEX idx_payment_webhooks_is_processed ON payment_webhooks(is_processed) WHERE NOT is_processed;
-- 冪等性唯一約束：同一 webhook_type + idempotency_key 只能有一筆已處理記錄
CREATE UNIQUE INDEX idx_payment_webhooks_idempotency
    ON payment_webhooks(webhook_type, idempotency_key)
    WHERE is_processed = TRUE;
-- 清理排程用索引：支援依處理時間清理已處理記錄
CREATE INDEX IF NOT EXISTS idx_payment_webhooks_cleanup
    ON payment_webhooks(processed_at)
    WHERE is_processed = TRUE;

COMMENT ON TABLE payment_webhooks IS 'Webhook 記錄表，支援多種 NotifyType 的冪等性檢查';

-- -----------------------------------------------------
-- 4. 操作與衝突日誌表 (payment_logs)
-- -----------------------------------------------------
CREATE TABLE payment_logs (
    id              BIGSERIAL PRIMARY KEY,              -- 自增主鍵（內部使用）
    trade_no        VARCHAR(32),                        -- 交易編號
    refund_no       VARCHAR(32),                        -- 退款編號（選填）

    -- 日誌資訊
    log_type        VARCHAR(50) NOT NULL,               -- 日誌類型
    log_level       VARCHAR(10) NOT NULL,               -- 日誌級別 (INFO, WARNING, ERROR)
    message         TEXT NOT NULL,                      -- 日誌內容

    -- 狀態變更
    from_status     VARCHAR(4),                          -- 變更前狀態
    to_status       VARCHAR(4),                          -- 變更後狀態
    trigger_source  VARCHAR(50),                        -- 觸發來源 (SYNC_API, WEBHOOK, TTL_CONSUMER, MANUAL)

    -- 衝突詳情（僅 CONFLICT 時使用）
    conflict_detail JSONB,                              -- 衝突詳情

    -- 操作者資訊（人工介入時使用）
    operator_id     VARCHAR(64),                        -- 操作人員
    operator_reason VARCHAR(500),                       -- 操作原因

    -- 時間戳記
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 約束
    CONSTRAINT chk_log_level CHECK (log_level IN ('INFO', 'WARNING', 'ERROR'))
);

CREATE INDEX idx_payment_logs_trade_no ON payment_logs(trade_no);
CREATE INDEX idx_payment_logs_refund_no ON payment_logs(refund_no);
CREATE INDEX idx_payment_logs_log_type ON payment_logs(log_type);
CREATE INDEX idx_payment_logs_created_at ON payment_logs(created_at);
-- 清理排程用索引：支援依時間+級別清理
CREATE INDEX IF NOT EXISTS idx_payment_logs_cleanup
    ON payment_logs(created_at, log_level);

COMMENT ON TABLE payment_logs IS '操作與衝突日誌表';
COMMENT ON COLUMN payment_logs.log_type IS '日誌類型：STATUS_CHANGE, CONFLICT, WEBHOOK_RECEIVED, WEBHOOK_INVALID, MANUAL_OVERRIDE, OUTBOX_FAILED';

-- -----------------------------------------------------
-- 5. Outbox 事件表（§2A.6 重複付款 + 退款事件用）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS outbox_events (
    id            BIGSERIAL PRIMARY KEY,
    event_type    VARCHAR(64)  NOT NULL,
    routing_key   VARCHAR(128) NOT NULL,
    payload       JSONB        NOT NULL,
    status        VARCHAR(20)  NOT NULL DEFAULT '8221',     -- OutboxEventStatus.PENDING
    retry_count   SMALLINT     NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ  NULL,
    sent_at       TIMESTAMPTZ  NULL,
    last_error    TEXT         NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_payment_outbox_status CHECK (status IN ('8221', '8222', '8223', '8224'))
);
-- 待重送掃描索引：PENDING(8221) + FAILED(8223)
CREATE INDEX idx_outbox_events_pending ON outbox_events(status, next_retry_at)
    WHERE status IN ('8221', '8223');
COMMENT ON TABLE outbox_events IS 'Outbox pattern：DB 與 MQ 一致性，由 OutboxPublisher 掃描 PENDING 發送';

-- -----------------------------------------------------
-- 6. 更新時間戳記觸發器
-- -----------------------------------------------------
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_payment_transactions_updated_at
    BEFORE UPDATE ON payment_transactions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trigger_payment_refunds_updated_at
    BEFORE UPDATE ON payment_refunds
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
