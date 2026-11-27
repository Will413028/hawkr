-- buyer 模組的 desired state（Atlas 與 sqlc 共用）。
-- public 不屬於本模組，宣告它以避免 Atlas 產生 DROP SCHEMA public。
-- 所有表位於 buyer；外部業務 ID 不建立跨 schema FK。
CREATE SCHEMA IF NOT EXISTS public;
CREATE SCHEMA buyer;

-- 買家：建帳時建立，一個帳號一生只有一列（帳號活著 ⇔ 買家活著，ADR 2026-09-30-account-created-provisions-buyer）；
-- 關閉與 PII 清理只作用於指定 buyer_id。
CREATE TABLE buyer.buyers (
    buyer_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     BIGINT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'ACTIVE',
    version        BIGINT NOT NULL DEFAULT 1,
    display_name   TEXT,
    phone          TEXT,
    gender         TEXT,
    avatar         TEXT,
    -- avatar 的 media asset ID（UUID v7）：換頭像在同一個 transaction 內 Confirm 新資產、MarkDeleted 被換掉的資產。
    avatar_asset_id UUID,
    birth_date     DATE,
    city           TEXT,
    job            TEXT,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    deleted_at     TIMESTAMPTZ,
    pii_cleared_at  TIMESTAMPTZ,
    CONSTRAINT buyers_status_check CHECK (status IN ('ACTIVE', 'PENDING_DELETION', 'CLOSED')),
    CONSTRAINT buyers_gender_check CHECK (gender IN ('MALE', 'FEMALE', 'OTHER')),
    CONSTRAINT buyers_deleted_check CHECK ((status = 'CLOSED') = (deleted_at IS NOT NULL)),
    CONSTRAINT buyers_pii_cleared_check CHECK (status = 'CLOSED' OR pii_cleared_at IS NULL),
    CONSTRAINT buyers_avatar_asset_check CHECK ((avatar IS NULL) = (avatar_asset_id IS NULL)),
    CONSTRAINT buyers_display_name_check CHECK (
        (pii_cleared_at IS NOT NULL AND display_name IS NULL) OR
        (pii_cleared_at IS NULL AND display_name IS NOT NULL AND char_length(display_name) BETWEEN 1 AND 100)
    )
);
CREATE UNIQUE INDEX buyers_account ON buyer.buyers (account_id);

-- 收貨地址：每個買家最多一個預設地址，刪除時直接移除個資。town_code 是內政部的鄉鎮市區代碼（8 碼，前 5 碼是縣市；
-- 表在 platform/twaddr，寫入端檢查是現行的代碼），縣市與郵遞區號由它推導，不另存。
CREATE TABLE buyer.shipping_addresses (
    address_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    buyer_id       BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    receiver_name  TEXT NOT NULL,
    receiver_phone TEXT NOT NULL,
    town_code      TEXT NOT NULL,
    address_detail TEXT NOT NULL,
    is_default     BOOLEAN NOT NULL,
    version        BIGINT NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT shipping_addresses_town_code_check CHECK (town_code ~ '^[0-9]{8}$'),
    CONSTRAINT shipping_addresses_address_detail_check CHECK (char_length(address_detail) BETWEEN 1 AND 255)
);
CREATE UNIQUE INDEX addresses_default ON buyer.shipping_addresses (buyer_id) WHERE is_default;

-- 收藏：買家與商品的關聯，以複合鍵保證重複新增冪等。
CREATE TABLE buyer.favorites (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    product_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, product_id)
);

-- 瀏覽紀錄：每組買家與商品只保留最近一次瀏覽時間。
CREATE TABLE buyer.visit_history (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    product_id UUID NOT NULL,
    visited_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, product_id)
);

-- 追蹤店家：關聯由 buyer 擁有，(store_id, buyer_id) 索引供店家粉絲查詢與依 buyer_id 的 keyset 分頁（追蹤者 fan-out）。
CREATE TABLE buyer.seller_subscriptions (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    store_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, store_id)
);
CREATE INDEX subscriptions_store ON buyer.seller_subscriptions (store_id, buyer_id);

-- 個人封鎖分兩張表，因為對象是兩種不同的東西：買家封鎖買家（對象在本模組，建外鍵）與買家封鎖店（店在 seller，不建跨模組外鍵）。
-- 買家 ID 與店家 ID 是兩個號碼空間，分表後不可能把買家 7 當成店 7。發起者買家刪帳時由刪帳流程刪掉他發起的兩種列，
-- 以及以他為對象的買家封鎖買家的列；以店為對象的列在店退場後保留。反向索引供「誰封鎖了這個對象」與買家間雙向查詢。
CREATE TABLE buyer.blocked_buyers (
    blocker_buyer_id BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    blocked_buyer_id BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    created_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (blocker_buyer_id, blocked_buyer_id),
    CONSTRAINT blocked_buyers_not_self_check CHECK (blocker_buyer_id <> blocked_buyer_id)
);
CREATE INDEX blocked_buyers_blocked ON buyer.blocked_buyers (blocked_buyer_id, blocker_buyer_id);

CREATE TABLE buyer.blocked_stores (
    blocker_buyer_id BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    store_id         BIGINT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (blocker_buyer_id, store_id),
    CONSTRAINT blocked_stores_store_id_check CHECK (store_id > 0)
);
CREATE INDEX blocked_stores_store ON buyer.blocked_stores (store_id, blocker_buyer_id);

-- 補貨訂閱：SKU 粒度的一次性訂閱，只保留待通知的買家與 SKU 關聯，取消或通知後刪除，不設到期。product_id 是 SKU 所屬的商品（顯示與清單用），
-- 商品與 SKU 在 catalog，不建 FK。
CREATE TABLE buyer.restock_subscriptions (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    sku_id     UUID NOT NULL,
    product_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, sku_id)
);
CREATE INDEX restock_sku ON buyer.restock_subscriptions (sku_id, buyer_id);

