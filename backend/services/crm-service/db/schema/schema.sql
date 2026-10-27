-- CRM Service Database Schema

-- 客戶關係表（多租戶：store_id=店家代理鍵, buyer_id=買家 buyer_id（buyer-service PK，非帳號終身 ID））
CREATE TABLE IF NOT EXISTS crm_customers (
    id                  BIGINT PRIMARY KEY,
    store_id            BIGINT NOT NULL,            -- 賣家 store_id
    buyer_id            BIGINT NOT NULL,            -- 買家 buyer_id（buyers.buyer_id）
    customer_type       VARCHAR(20) NOT NULL DEFAULT 'normal', -- normal, vip, blocklist
    total_spent         BIGINT NOT NULL DEFAULT 0,
    order_count         INT NOT NULL DEFAULT 0,
    nickname            VARCHAR(100) NOT NULL DEFAULT '',  -- cache from account-service
    phone               VARCHAR(20) NOT NULL DEFAULT '',   -- cache from account-service
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (store_id, buyer_id)
);

COMMENT ON COLUMN crm_customers.buyer_id IS 'buyers.buyer_id (per-stint PK, never equals account_id). Renamed from customer_account_id 2026-07-26.';

CREATE INDEX IF NOT EXISTS idx_crm_customers_store_id ON crm_customers (store_id);
CREATE INDEX IF NOT EXISTS idx_crm_customers_customer_type ON crm_customers (store_id, customer_type);

-- 標籤表（賣家自定義，多租戶）
CREATE TABLE IF NOT EXISTS crm_tags (
    id         BIGINT PRIMARY KEY,
    store_id   BIGINT NOT NULL,    -- 賣家 store_id
    name       VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_crm_tags_store_id ON crm_tags (store_id);

-- 客戶-標籤關聯表
CREATE TABLE IF NOT EXISTS crm_customer_tags (
    id                  BIGSERIAL PRIMARY KEY,
    store_id            BIGINT NOT NULL,
    buyer_id            BIGINT NOT NULL,
    tag_id              BIGINT NOT NULL REFERENCES crm_tags(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (store_id, buyer_id, tag_id)
);

COMMENT ON COLUMN crm_customer_tags.buyer_id IS 'buyers.buyer_id. Renamed from customer_account_id 2026-07-26.';

CREATE INDEX IF NOT EXISTS idx_crm_customer_tags_customer ON crm_customer_tags (store_id, buyer_id);
CREATE INDEX IF NOT EXISTS idx_crm_customer_tags_tag ON crm_customer_tags (tag_id);

-- 客戶消費/退款明細帳（insert-only ledger；event_id 為冪等鍵，取代 crm_processed_orders。
-- 見 docs/superpowers/specs/2026-07-25-crm-customer-ledger-design.md）
CREATE TABLE IF NOT EXISTS crm_customer_ledger (
    event_id             VARCHAR(64) PRIMARY KEY,
    store_id             BIGINT NOT NULL,
    buyer_id             BIGINT NOT NULL,
    event_type           VARCHAR(30) NOT NULL,  -- 'order_completed' | 'refund_completed_full' | 'refund_completed_partial'
    order_no             VARCHAR(64) NOT NULL,
    amount_delta         BIGINT NOT NULL,
    count_delta          INT NOT NULL,
    occurred_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON COLUMN crm_customer_ledger.buyer_id IS 'buyers.buyer_id. Renamed from customer_account_id 2026-07-26.';

CREATE INDEX IF NOT EXISTS idx_crm_customer_ledger_customer ON crm_customer_ledger (store_id, buyer_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_crm_customer_ledger_order_no ON crm_customer_ledger (order_no);
