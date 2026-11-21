-- crm 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（crm.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA crm;

-- 店家對買家的分類與消費統計。主鍵是業務鍵的自然鍵（row-identity ADR：關係列用自然鍵）；沒有暱稱、電話等個資欄（A6-D8），
-- 暱稱由 httpapi 經 buyer.BatchDisplay 讀時補。客戶只在首筆完成訂單時建立（退款不建）。
-- total_spent 恆等於該客戶 completed 訂單的 SUM(GREATEST(net_amount, 0))（design §3.3）；order_count 是完成訂單數，退款不扣。
CREATE TABLE crm.customers (
    store_id       BIGINT      NOT NULL,
    buyer_id       BIGINT      NOT NULL,
    customer_type  TEXT        NOT NULL DEFAULT 'NORMAL',
    total_spent    BIGINT      NOT NULL DEFAULT 0,
    order_count    INTEGER     NOT NULL DEFAULT 0,
    first_order_at TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id),
    CONSTRAINT customers_store_check CHECK (store_id > 0),
    CONSTRAINT customers_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT customers_type_check CHECK (customer_type IN ('NORMAL', 'VIP')),
    CONSTRAINT customers_total_spent_check CHECK (total_spent >= 0),
    CONSTRAINT customers_order_count_check CHECK (order_count >= 0)
);
CREATE INDEX customers_by_spent ON crm.customers (store_id, total_spent DESC, buyer_id DESC);
CREATE INDEX customers_by_orders ON crm.customers (store_id, order_count DESC, buyer_id DESC);
CREATE INDEX customers_by_first_order ON crm.customers (store_id, first_order_at DESC, buyer_id DESC);
-- 必要條件：BuyerDeleted 的抹除依 buyer_id 刪跨店的列（W2 (b)）。
CREATE INDEX customers_by_buyer ON crm.customers (buyer_id);

-- 店家的標籤。(store_id, name) 唯一（CD3）；(store_id, id) 唯一是 customer_tags 複合 FK 的目標（CD4：標籤與客戶同店由 DB 保證）。
CREATE TABLE crm.tags (
    id         UUID        PRIMARY KEY,
    store_id   BIGINT      NOT NULL,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT tags_store_name_key UNIQUE (store_id, name),
    CONSTRAINT tags_store_id_key UNIQUE (store_id, id),
    CONSTRAINT tags_store_check CHECK (store_id > 0),
    CONSTRAINT tags_name_check CHECK (char_length(name) >= 1 AND char_length(name) <= 50 AND name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);

CREATE TABLE crm.customer_tags (
    store_id   BIGINT      NOT NULL,
    buyer_id   BIGINT      NOT NULL,
    tag_id     UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id, tag_id),
    CONSTRAINT customer_tags_tag_fkey FOREIGN KEY (store_id, tag_id) REFERENCES crm.tags (store_id, id) ON DELETE CASCADE,
    CONSTRAINT customer_tags_customer_fkey FOREIGN KEY (store_id, buyer_id) REFERENCES crm.customers (store_id, buyer_id) ON DELETE CASCADE
);
CREATE INDEX customer_tags_by_tag ON crm.customer_tags (store_id, tag_id, buyer_id);

-- 消費統計的投影（過渡：ordering 搬進單體後改讀 ordering 的 pub_*，這兩張表退場）。每單一列，列鎖取代舊的 advisory lock；
-- net_amount 可暫時為負（退款先到）；只有 completed 的單計入 customers.total_spent。
CREATE TABLE crm.order_nets (
    order_no      TEXT        PRIMARY KEY,
    store_id      BIGINT      NOT NULL,
    buyer_id      BIGINT      NOT NULL,
    completed     BOOLEAN     NOT NULL DEFAULT false,
    -- 這單是否收過全額退款（在已上鎖的列上讀寫；訂單比全額退款晚到時記 0）。
    full_refunded BOOLEAN     NOT NULL DEFAULT false,
    net_amount    BIGINT      NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT order_nets_order_no_check CHECK (char_length(order_no) >= 1 AND char_length(order_no) <= 128),
    CONSTRAINT order_nets_store_check CHECK (store_id > 0),
    CONSTRAINT order_nets_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX order_nets_by_customer ON crm.order_nets (store_id, buyer_id);
CREATE INDEX order_nets_by_buyer ON crm.order_nets (buyer_id);

-- 已處理的事件。冪等靠 dedup_key 的 UNIQUE（order:<event_id>、refund:<external_ref_id 或 AS_REFUND_<case_no>>）。
-- 只追加的紀錄，主鍵是 DB 產生的 UUIDv7（row-identity ADR D1）；刪帳時隨 order_nets 連帶刪除。
CREATE TABLE crm.ledger (
    id        UUID PRIMARY KEY DEFAULT uuidv7(),
    dedup_key TEXT NOT NULL,
    order_no  TEXT NOT NULL REFERENCES crm.order_nets (order_no) ON DELETE CASCADE,
    CONSTRAINT ledger_dedup_key_key UNIQUE (dedup_key),
    CONSTRAINT ledger_dedup_key_check CHECK (char_length(dedup_key) >= 1 AND char_length(dedup_key) <= 128)
);
