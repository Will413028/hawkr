-- ============================================================
-- AfterSales Service V1 Migration
-- DB: hawkr_aftersales
-- ============================================================

-- 共用 trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- ============================================================
-- 1. aftersale_cases（售後單主表）
-- ============================================================

CREATE TABLE aftersale_cases (
    id                     BIGINT       PRIMARY KEY,
    case_no                VARCHAR(64)  NOT NULL,
    order_no               VARCHAR(64)  NOT NULL,
    buyer_account_id       BIGINT       NOT NULL,
    seller_account_id      BIGINT       NOT NULL,
    status                 VARCHAR(4)   NOT NULL DEFAULT '0',
    reason                 VARCHAR(20)  NOT NULL,
    description            TEXT         NOT NULL,
    refundable_amount      BIGINT       NOT NULL,
    shipping_fee           BIGINT       NOT NULL DEFAULT 0,
    delivery_temp          VARCHAR(20),
    buyer_name             VARCHAR(50),
    buyer_phone            VARCHAR(20),
    buyer_city             VARCHAR(20),
    buyer_district         VARCHAR(20),
    buyer_address_detail   VARCHAR(200),
    buyer_postal_code      VARCHAR(10),
    transaction_id         VARCHAR(128),
    payment_method         VARCHAR(30),
    mer_id                 VARCHAR(30),
    shipping_method        VARCHAR(30),
    reopen_count           SMALLINT     NOT NULL DEFAULT 0,
    max_reopen_count       SMALLINT     NOT NULL DEFAULT 1,
    return_shipment_no     VARCHAR(64),
    return_shipping_method VARCHAR(30),
    ship_trade_no          VARCHAR(64),
    partner_id             VARCHAR(10),
    odno                   VARCHAR(20),
    validation_no          VARCHAR(10),
    label_retry_count      SMALLINT     NOT NULL DEFAULT 0,
    return_delivered_at    TIMESTAMPTZ,                          -- 退貨送達賣家時間（PayUni webhook 觸發 25→27 或 26→27 流轉時 stamp）
    auto_confirm_at        TIMESTAMPTZ,                          -- 自動確認收貨截止時間（= return_delivered_at + AUTO_CONFIRM_RECEIPT_HOURS；scheduler 掃描用）
    return_provider_status VARCHAR(20),                          -- PayUni 退貨物流原始狀態碼快照（status=28 RETURN_FAILED 時填入；2026-05-22 起碼 55 賣家未取也會 stamp 但不變狀態）
    return_provider_message TEXT,                                -- PayUni 解密後錯誤訊息原文快照
    -- 【2026-05-22 逆物流退貨政策】超商「到店待取」與「自動完成原因持久化」與「24h 預警」相關欄位
    store_arrived_at              TIMESTAMPTZ,                   -- 超商退貨到店時間（PayUni 碼 32 觸發 25→26 流轉時 stamp）
    store_pickup_deadline_at      TIMESTAMPTZ,                   -- 超商待取逾時截止（= store_arrived_at + STORE_PICKUP_TIMEOUT_HOURS；StorePickupTimeoutScheduler 掃此欄位）
    seller_picked_up_at           TIMESTAMPTZ,                   -- 超商賣家實際取貨時間（PayUni 碼 11 觸發 26→27 流轉時 stamp）
    auto_complete_at_stamp        TIMESTAMPTZ,                   -- 系統實際自動完成時間戳（status=31/32 時 stamp；原因由 status 直接表達，不另存 reason metadata）
    warned_store_pickup_24h_at    TIMESTAMPTZ,                   -- 超商待取剩 24h 預警 FCM 已發時間戳（NULL=未發；ReturnTwentyFourHourWarnScheduler 冪等用）
    warned_confirm_receipt_24h_at TIMESTAMPTZ,                   -- 確認收貨剩 24h 預警 FCM 已發時間戳
    delivered_at           TIMESTAMPTZ,
    closed_at              TIMESTAMPTZ,
    -- §10 申訴與平台裁決
    appeal_state           VARCHAR(4)   NOT NULL DEFAULT '0',
    appeal_submitted_at    TIMESTAMPTZ,
    appeal_resolved_at     TIMESTAMPTZ,
    last_rejected_at       TIMESTAMPTZ,
    -- §10.6.6 申訴內容結構化儲存（供 Admin 詳情 API 讀取，不再解析 events.description 字串）
    appeal_reason          VARCHAR(128),
    appeal_description     TEXT,
    -- §5.10 賣家處理期限自動升級（process timeout auto-escalation）
    process_deadline       TIMESTAMPTZ,                          -- 僅 status=PENDING/REOPENED 時有值
    auto_escalated         BOOLEAN      NOT NULL DEFAULT false,  -- 系統自動升級至 admin 仲裁
    escalated_at           TIMESTAMPTZ,                          -- 自動升級時間戳
    created_at             TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ    DEFAULT NOW()
);

CREATE UNIQUE INDEX uk_aftersale_cases_case_no ON aftersale_cases (case_no);
CREATE INDEX idx_aftersale_cases_order_no ON aftersale_cases (order_no);
CREATE INDEX idx_aftersale_cases_buyer ON aftersale_cases (buyer_account_id, status);
CREATE INDEX idx_aftersale_cases_seller ON aftersale_cases (seller_account_id, status);
CREATE INDEX idx_aftersale_cases_return_timeout ON aftersale_cases (created_at)
    WHERE status = '20';

-- §5.3.10.1 AutoConfirmReceiptScheduler 掃描（每 10 分鐘）
CREATE INDEX idx_aftersale_cases_auto_confirm ON aftersale_cases (auto_confirm_at)
    WHERE status = '27' AND auto_confirm_at IS NOT NULL;

-- §5.3.10.2 StorePickupTimeoutScheduler 掃描（2026-05-22 新增，每 10 分鐘）
CREATE INDEX idx_aftersale_cases_store_pickup_deadline ON aftersale_cases (store_pickup_deadline_at)
    WHERE status = '26' AND store_pickup_deadline_at IS NOT NULL;

-- §5.3.10.3 ReturnTwentyFourHourWarnScheduler 掃描（2026-05-22 新增，每 5 分鐘）
CREATE INDEX idx_aftersale_cases_warn_store_24h ON aftersale_cases (store_pickup_deadline_at)
    WHERE status = '26' AND warned_store_pickup_24h_at IS NULL;
CREATE INDEX idx_aftersale_cases_warn_confirm_24h ON aftersale_cases (auto_confirm_at)
    WHERE status = '27' AND warned_confirm_receipt_24h_at IS NULL;

-- §5.10 賣家處理逾時掃描（CaseProcessTimeoutWorker）
CREATE INDEX idx_aftersale_cases_process_deadline ON aftersale_cases (process_deadline)
    WHERE process_deadline IS NOT NULL AND auto_escalated = false;

-- 7-11 退貨出貨便代碼重試掃描（§5.3.7 ReturnLabelRetryWorker / ReturnLabelTimeoutWorker）
CREATE INDEX idx_aftersale_cases_label_retry ON aftersale_cases (created_at, label_retry_count)
    WHERE status IN ('20', '25') AND return_shipping_method = '3002' AND partner_id IS NULL;

-- 同一訂單同時只能有一筆非終態售後單
CREATE UNIQUE INDEX uk_aftersale_cases_active_order ON aftersale_cases (order_no)
    WHERE status NOT IN ('50', '70');

-- §10 Admin 待裁決列表查詢（appeal_state=APPEALED 排序 appeal_submitted_at）
CREATE INDEX idx_aftersale_cases_appeal_state ON aftersale_cases (appeal_state, appeal_submitted_at)
    WHERE appeal_state <> '0';

DROP TRIGGER IF EXISTS update_aftersale_cases_updated_at ON aftersale_cases;
CREATE TRIGGER update_aftersale_cases_updated_at
    BEFORE UPDATE ON aftersale_cases
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 2. aftersale_case_items（售後單明細）
-- ============================================================

CREATE TABLE aftersale_case_items (
    id                 BIGINT       PRIMARY KEY,
    case_no            VARCHAR(64)  NOT NULL,
    order_item_id      BIGINT       NOT NULL,
    sku_id             BIGINT       NOT NULL,
    sku_name           VARCHAR(255) NOT NULL,
    quantity           INTEGER      NOT NULL,
    original_price     BIGINT       NOT NULL,
    actual_pay_amount  BIGINT       NOT NULL,
    image_object_key   VARCHAR(512),
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aftersale_case_items_case_no ON aftersale_case_items (case_no);

-- ============================================================
-- 3. aftersale_evidence（證據照片）
-- ============================================================

CREATE TABLE aftersale_evidence (
    id             BIGINT       PRIMARY KEY,
    case_no        VARCHAR(64)  NOT NULL,
    asset_id       BIGINT       NOT NULL,
    object_key     VARCHAR(512) NOT NULL,
    uploaded_by    BIGINT       NOT NULL,
    evidence_type  VARCHAR(20)  NOT NULL, -- AfterSaleEvidenceType value（8601=PHOTO 買家申請/補件、8602=INSPECTION_PHOTO 賣家驗貨）
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aftersale_evidence_case_no ON aftersale_evidence (case_no);

-- ============================================================
-- 4. aftersale_resolutions（賣家決策記錄）
-- ============================================================

CREATE TABLE aftersale_resolutions (
    id                      BIGINT       PRIMARY KEY,
    case_no                 VARCHAR(64)  NOT NULL,
    resolution_type         VARCHAR(30)  NOT NULL,
    resolution_source       VARCHAR(16)  NOT NULL DEFAULT 'SELLER',
    refund_amount           BIGINT,
    coupon_amount           BIGINT,
    coupon_min_spend        BIGINT,
    coupon_message          VARCHAR(500),
    coupon_template_id      VARCHAR(64),
    reject_reason           VARCHAR(30),
    reject_message          TEXT,
    negotiation_reason      VARCHAR(500),
    return_shipping_method  VARCHAR(30),
    deduction_reason        VARCHAR(200),
    inspection_note         TEXT,
    -- §5.10 AUTO_REJECT 路徑（worker 系統代為拒絕）寫 NULL；其他人類操作（賣家手動 / Admin 裁決）必填
    resolved_by             BIGINT,
    created_at              TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aftersale_resolutions_case_no ON aftersale_resolutions (case_no);

-- ============================================================
-- 5. refund_tasks（退款執行任務）
-- ============================================================

CREATE TABLE refund_tasks (
    id                     BIGINT        PRIMARY KEY,
    case_no                VARCHAR(64),
    cancel_id              BIGINT,
    order_no               VARCHAR(64)   NOT NULL,
    transaction_id         VARCHAR(128)  NOT NULL,
    refund_amount          BIGINT        NOT NULL,
    payment_method         VARCHAR(30)   NOT NULL,
    mer_id                 VARCHAR(30)   NOT NULL,
    status                 VARCHAR(4)    NOT NULL DEFAULT '0',
    external_ref_id        VARCHAR(128),
    last_error             TEXT,
    retry_count            INTEGER       NOT NULL DEFAULT 0,
    max_retry_count        INTEGER       NOT NULL DEFAULT 10,
    next_retry_time        TIMESTAMPTZ,
    payuni_status          VARCHAR(20),
    payuni_message         TEXT,
    estimated_refund_date  DATE,
    bank_date              DATE,
    client_type            VARCHAR(16)   NOT NULL DEFAULT 'MOBILE_APP',  -- §5.4.2 CodeMeta ClientType value（MOBILE_APP / WEB_BROWSER）
    app_type               VARCHAR(8)    NOT NULL DEFAULT 'BUYER',       -- §5.4.2 CodeMeta AppType value（BUYER / SELLER）
    current_attempt        SMALLINT      NOT NULL DEFAULT 1,             -- §2.2 業務 pivot (2026-05-14)：當前對 PayUni 發 RFD 的 attempt 序號；失敗退回 5 時 +1
    created_at             TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ   DEFAULT NOW(),

    CONSTRAINT chk_refund_tasks_source CHECK (
        (case_no IS NOT NULL AND cancel_id IS NULL) OR
        (case_no IS NULL AND cancel_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX uk_refund_tasks_case_no ON refund_tasks (case_no) WHERE case_no IS NOT NULL;
CREATE UNIQUE INDEX uk_refund_tasks_cancel_id ON refund_tasks (cancel_id) WHERE cancel_id IS NOT NULL;
CREATE INDEX idx_refund_tasks_status_retry ON refund_tasks (status, next_retry_time);
CREATE INDEX idx_refund_tasks_order_no ON refund_tasks (order_no);
CREATE INDEX idx_refund_tasks_cleanup ON refund_tasks (updated_at)
    WHERE status IN ('20', '40');

DROP TRIGGER IF EXISTS update_refund_tasks_updated_at ON refund_tasks;
CREATE TRIGGER update_refund_tasks_updated_at
    BEFORE UPDATE ON refund_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 6. seller_cancel_requests（取消出貨記錄）
-- ============================================================

CREATE TABLE seller_cancel_requests (
    id                 BIGINT       PRIMARY KEY,
    order_no           VARCHAR(64)  NOT NULL,
    seller_account_id  BIGINT       NOT NULL,
    buyer_account_id   BIGINT       NOT NULL,
    reason             VARCHAR(200) NOT NULL,
    remark             VARCHAR(500),
    status             VARCHAR(4)   NOT NULL DEFAULT '0',
    refund_amount      BIGINT       NOT NULL,
    transaction_id     VARCHAR(128) NOT NULL,
    payment_method     VARCHAR(30)  NOT NULL,
    mer_id             VARCHAR(30)  NOT NULL,
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ    DEFAULT NOW()
);

CREATE UNIQUE INDEX uk_seller_cancel_order_no ON seller_cancel_requests (order_no);
CREATE INDEX idx_seller_cancel_seller ON seller_cancel_requests (seller_account_id);
CREATE INDEX idx_seller_cancel_status ON seller_cancel_requests (status);

DROP TRIGGER IF EXISTS update_seller_cancel_requests_updated_at ON seller_cancel_requests;
CREATE TRIGGER update_seller_cancel_requests_updated_at
    BEFORE UPDATE ON seller_cancel_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ============================================================
-- 7. aftersale_audit_logs（操作歷程 / 稽核表）
-- ============================================================

CREATE TABLE aftersale_audit_logs (
    id                 BIGSERIAL    PRIMARY KEY,
    case_no            VARCHAR(64),
    cancel_id          BIGINT,
    actor_account_id   BIGINT,                     -- SYSTEM 操作時可為 NULL
    actor_type         VARCHAR(20)  NOT NULL, -- AfterSaleAuditActor value（8401=BUYER / 8402=SELLER / 8403=SYSTEM）
    action             VARCHAR(50)  NOT NULL, -- AfterSaleAuditAction value（8501-8520，詳見內部技術碼.md）
    detail             JSONB,
    created_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_case_no ON aftersale_audit_logs (case_no) WHERE case_no IS NOT NULL;
CREATE INDEX idx_audit_logs_cancel_id ON aftersale_audit_logs (cancel_id) WHERE cancel_id IS NOT NULL;
CREATE INDEX idx_audit_logs_created_at ON aftersale_audit_logs (created_at);

-- ============================================================
-- 8. aftersale_platform_decisions（平台裁決記錄，§10.4.3）
-- ============================================================

CREATE TABLE aftersale_platform_decisions (
    id                   BIGSERIAL    PRIMARY KEY,
    case_no              VARCHAR(64)  NOT NULL,
    outcome              VARCHAR(30)  NOT NULL, -- AfterSalePlatformDecisionOutcome value（FULL_REFUND / PARTIAL_REFUND / REJECT_APPEAL）
    refund_amount        BIGINT,                -- 僅 PARTIAL_REFUND 時填入
    reason               TEXT         NOT NULL,
    decided_by_admin_id  BIGINT       NOT NULL,
    decided_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uk_aftersale_platform_decisions_case_no ON aftersale_platform_decisions (case_no);
CREATE INDEX idx_aftersale_platform_decisions_admin ON aftersale_platform_decisions (decided_by_admin_id, decided_at);

-- ============================================================
-- 9. aftersale_appeal_events（申訴事件流水，§10.4.4）
-- ============================================================

CREATE TABLE aftersale_appeal_events (
    id           BIGSERIAL    PRIMARY KEY,
    case_no      VARCHAR(64)  NOT NULL,
    event_type   VARCHAR(30)  NOT NULL, -- AfterSaleAppealEventType value（APPEAL_SUBMITTED / DECISION_MADE）
    actor_type   VARCHAR(20)  NOT NULL, -- BUYER / ADMIN / SYSTEM
    actor_id     BIGINT,                -- SYSTEM 事件為 NULL
    label        VARCHAR(64)  NOT NULL,
    description  TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_aftersale_appeal_events_case_no ON aftersale_appeal_events (case_no, created_at);
