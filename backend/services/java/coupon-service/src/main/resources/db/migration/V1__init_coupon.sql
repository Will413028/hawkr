-- =====================================================
-- Coupon Service 資料庫初始化腳本
-- 依據優惠券微服務詳細設計書 §4 定義
-- =====================================================

-- 建立 updated_at 自動更新觸發器函數
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- -----------------------------------------------------
-- 1. 優惠券定義表（設計書 §4.1）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS coupon_templates (
    template_id BIGINT PRIMARY KEY, -- TSID
    title VARCHAR(255) NOT NULL, -- 優惠券名稱
    type VARCHAR(4) NOT NULL, -- value: 1=CASH / 2=PERCENT / 3=FREE_SHIPPING
    applicable_seller_account_id BIGINT, -- NULL = 全平台券
    issuer_type VARCHAR(4) NOT NULL, -- value: 1=ADMIN / 2=SELLER
    issuer_account_id BIGINT NOT NULL, -- 建立者 accountId
    threshold_amount BIGINT NOT NULL DEFAULT 0, -- 滿額門檻（新台幣元），0 = 無門檻
    discount_amount BIGINT, -- CASH 券折扣額（新台幣元）
    discount_percent INTEGER, -- PERCENT 券折扣百分比（如 20 = 打 8 折）
    max_discount_amount BIGINT, -- PERCENT 券折扣上限（新台幣元）
    max_shipping_discount BIGINT, -- FREE_SHIPPING 券免運上限（新台幣元）
    start_at TIMESTAMPTZ NOT NULL, -- 生效時間
    end_at TIMESTAMPTZ NOT NULL, -- 到期時間
    usage_limit_per_account INTEGER NOT NULL DEFAULT 1, -- 每帳號領取上限
    total_issue_limit INTEGER NOT NULL DEFAULT -1, -- 總發行上限，-1 = 無限
    issued_count INTEGER NOT NULL DEFAULT 0, -- 已發行數量
    target_type VARCHAR(4) NOT NULL, -- value: 1=ALL / 2=MEMBER / 3=DESIGNATED / 4=COMPENSATION
    status VARCHAR(4) NOT NULL DEFAULT '0', -- value: 0=DRAFT / 1=ACTIVE / 2=INACTIVE / 3=EXPIRED / 4=REVOKED
    claim_required BOOLEAN NOT NULL DEFAULT TRUE, -- 是否需要領券（Seller/補償券強制 FALSE）
    revoked_at TIMESTAMPTZ,
    revoked_reason VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tpl_scope_status ON coupon_templates (
    applicable_seller_account_id,
    status
);

CREATE INDEX IF NOT EXISTS idx_tpl_issuer ON coupon_templates (
    issuer_type,
    issuer_account_id,
    status
);

CREATE INDEX IF NOT EXISTS idx_tpl_end_at ON coupon_templates (end_at)
WHERE
    status IN ('1', '2');

CREATE INDEX IF NOT EXISTS idx_tpl_target_type ON coupon_templates (target_type, status);

CREATE INDEX IF NOT EXISTS idx_tpl_claim ON coupon_templates (claim_required, status)
WHERE
    claim_required = TRUE
    AND status = '1';

DROP TRIGGER IF EXISTS update_coupon_templates_updated_at ON coupon_templates;

CREATE TRIGGER update_coupon_templates_updated_at
    BEFORE UPDATE ON coupon_templates
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- -----------------------------------------------------
-- 2. 優惠券實體表（設計書 §4.2）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS coupon_instances (
    instance_id BIGINT PRIMARY KEY, -- TSID
    template_id BIGINT NOT NULL, -- FK → coupon_templates
    owner_account_id BIGINT NOT NULL, -- 持有者（買家）accountId
    applicable_seller_account_id BIGINT, -- 冗餘自 Template，加速查詢
    status VARCHAR(4) NOT NULL DEFAULT '0', -- value: 0=UNUSED / 1=RESERVED / 2=USED / 3=EXPIRED / 4=REVOKED / 5=ANNUL
    reserved_order_no VARCHAR(64), -- 鎖定的訂單編號
    reserved_at TIMESTAMPTZ,
    used_order_no VARCHAR(64), -- 使用時的訂單編號
    used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_reason VARCHAR(255),
    voided_at TIMESTAMPTZ,
    voided_reason VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inst_owner_status ON coupon_instances (owner_account_id, status);

CREATE INDEX IF NOT EXISTS idx_inst_reserved_order ON coupon_instances (reserved_order_no)
WHERE
    reserved_order_no IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_inst_template ON coupon_instances (template_id, status);

CREATE INDEX IF NOT EXISTS idx_inst_seller_owner ON coupon_instances (
    applicable_seller_account_id,
    owner_account_id,
    status
);

-- 同一模板、同一買家只能持有一張有效券（排除 ANNUL 和 REVOKED）
CREATE UNIQUE INDEX IF NOT EXISTS uk_inst_template_owner ON coupon_instances (template_id, owner_account_id)
WHERE
    status NOT IN ('5', '4');

DROP TRIGGER IF EXISTS update_coupon_instances_updated_at ON coupon_instances;

CREATE TRIGGER update_coupon_instances_updated_at
    BEFORE UPDATE ON coupon_instances
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- -----------------------------------------------------
-- 3. 稽核日誌表（設計書 §4.3）— 內部表，BIGSERIAL 主鍵
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS coupon_redemption_logs (
    log_id BIGSERIAL PRIMARY KEY,
    instance_id BIGINT NOT NULL,
    template_id BIGINT NOT NULL,
    owner_account_id BIGINT NOT NULL,
    action VARCHAR(16) NOT NULL, -- CouponRedemptionAction value（8301=ISSUE / 8302=RECEIVE / 8303=RESERVE / 8304=CONSUME / 8305=RELEASE / 8306=REVOKE / 8307=ANNUL / 8308=EXPIRE）
    order_no VARCHAR(64),
    operator_account_id BIGINT, -- Admin/Seller 操作時記錄
    trace_id VARCHAR(64) NOT NULL DEFAULT '',
    request_id VARCHAR(64) NOT NULL DEFAULT '',
    detail VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_log_instance ON coupon_redemption_logs (instance_id);

CREATE INDEX IF NOT EXISTS idx_log_created_at ON coupon_redemption_logs (created_at);

-- -----------------------------------------------------
-- 4. 賣家配額表（設計書 §4.4）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS seller_coupon_quotas (
    seller_account_id BIGINT PRIMARY KEY,
    max_template_count INTEGER NOT NULL DEFAULT 5, -- 預設 5 種
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_seller_coupon_quotas_updated_at ON seller_coupon_quotas;

CREATE TRIGGER update_seller_coupon_quotas_updated_at
    BEFORE UPDATE ON seller_coupon_quotas
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- -----------------------------------------------------
-- 5. 發放目標帳號表（設計書 §4.5）
-- 僅 target_type = DESIGNATED / COMPENSATION 時使用
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS coupon_target_accounts (
    template_id BIGINT NOT NULL, -- FK → coupon_templates
    account_id BIGINT NOT NULL, -- 買家 accountId
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (template_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_target_account ON coupon_target_accounts (account_id);

-- -----------------------------------------------------
-- 6. 領券記錄表（設計書 §3.6）
-- 僅 claim_required = TRUE 的 Admin 優惠券使用
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS coupon_claims (
    template_id BIGINT NOT NULL, -- FK → coupon_templates
    buyer_account_id BIGINT NOT NULL, -- 領券的買家 accountId
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (template_id, buyer_account_id)
);

CREATE INDEX IF NOT EXISTS idx_claim_buyer ON coupon_claims (buyer_account_id);

COMMENT ON TABLE coupon_templates IS '優惠券定義表 - 設計書 §3.1';

COMMENT ON TABLE coupon_instances IS '優惠券實體表 - 設計書 §3.2';

COMMENT ON TABLE coupon_redemption_logs IS '稽核日誌表 - 設計書 §3.3';

COMMENT ON TABLE seller_coupon_quotas IS '賣家配額表 - 設計書 §3.4';

COMMENT ON TABLE coupon_target_accounts IS '發放目標帳號表 - 設計書 §3.5';

COMMENT ON TABLE coupon_claims IS '領券記錄表 - 設計書 §3.6';