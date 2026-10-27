-- =====================================================
-- Cart Service 資料庫初始化腳本
-- 依據購物車微服務詳細設計書 §3 定義
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
-- 1. 購物車明細表（設計書 §3.1）
-- 設計決策：不建立 carts 主表，直接以 account_id/guest_id 聚合
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS cart_items (
    id              BIGINT          PRIMARY KEY,                -- TSID
    account_id      BIGINT,                                     -- 帳號 ID（與 guest_id 擇一必填；metadata 注入）
    guest_id        VARCHAR(64),                                -- 訪客 UUID（與 account_id 擇一必填；前端生成）
    seller_account_id BIGINT        NOT NULL,                   -- 賣家帳號 ID（Product Service 取得，用於分組）
    sku_id          BIGINT          NOT NULL,                   -- 商品 SKU ID（= product_spec.id）
    quantity        INT             NOT NULL,                   -- 數量
    added_unit_price BIGINT         NOT NULL,                   -- 加入時實際成交單價快照（新台幣元，無特價=list_price，有特價=當時 sale_price）
    added_list_price BIGINT         NOT NULL,                   -- 加入時原價快照（新台幣元，永遠 = SKU.price，給前端 strike-through 顯示）
    max_limit_snapshot INT          NOT NULL,                   -- 加入時限購上限快照（= stock_qty）
    sku_name_snapshot VARCHAR(255)  NOT NULL,                   -- 加入時商品名稱快照（展示用）
    product_id      BIGINT          NOT NULL DEFAULT 0,         -- 商品 ID（AddItem 時從 ProductSimpleInfo 取得）
    spec_name_snapshot VARCHAR(255) NOT NULL DEFAULT '',        -- SKU 規格名稱快照（product_sku.sku_name）
    image_object_key VARCHAR(512)   NOT NULL DEFAULT '',        -- 商品主圖 objectKey 快照（GetCart 時組裝 CDN URL）
    status          VARCHAR(4)      NOT NULL DEFAULT '0',        -- 0=NORMAL, 1=LOCKED
    checkout_id     VARCHAR(64),                                -- 結帳批次 ID（鎖定時寫入）
    lock_expire_time TIMESTAMPTZ,                               -- 鎖定過期時間
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    -- 數量必須為正整數
    CONSTRAINT chk_quantity_positive CHECK (quantity > 0),

    -- 狀態只允許 0（NORMAL）或 1（LOCKED）
    CONSTRAINT chk_status_valid CHECK (status IN ('0', '1')),

    -- XOR 約束：account_id 與 guest_id 必須且只能擇一
    CONSTRAINT chk_owner_xor CHECK (
        (account_id IS NOT NULL AND guest_id IS NULL) OR
        (account_id IS NULL AND guest_id IS NOT NULL)
    )
);

-- 帳號 SKU 唯一約束（Partial Unique Index）
CREATE UNIQUE INDEX IF NOT EXISTS uk_cart_account_sku
    ON cart_items (account_id, sku_id) WHERE account_id IS NOT NULL;

-- 訪客 SKU 唯一約束（Partial Unique Index）
CREATE UNIQUE INDEX IF NOT EXISTS uk_cart_guest_sku
    ON cart_items (guest_id, sku_id) WHERE guest_id IS NOT NULL;

-- 結帳清理用索引
CREATE INDEX IF NOT EXISTS idx_cart_checkout_id
    ON cart_items (checkout_id) WHERE checkout_id IS NOT NULL;

-- 背景 Job 掃描過期鎖定用索引
CREATE INDEX IF NOT EXISTS idx_cart_lock_expire
    ON cart_items (lock_expire_time) WHERE status = '1' AND lock_expire_time IS NOT NULL;

-- 訪客清理 Job 用索引
CREATE INDEX IF NOT EXISTS idx_cart_guest_updated
    ON cart_items (updated_at) WHERE guest_id IS NOT NULL;

-- 賣家分組查詢用索引
CREATE INDEX IF NOT EXISTS idx_cart_account_seller
    ON cart_items (account_id, seller_account_id) WHERE account_id IS NOT NULL;

-- updated_at 自動更新觸發器
DROP TRIGGER IF EXISTS update_cart_items_updated_at ON cart_items;
CREATE TRIGGER update_cart_items_updated_at
    BEFORE UPDATE ON cart_items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE cart_items IS '購物車明細表 - 設計書 §3.1';
COMMENT ON COLUMN cart_items.added_unit_price IS '加入時實際成交單價快照（新台幣元，無特價=list_price，有特價=當時 sale_price）';
COMMENT ON COLUMN cart_items.added_list_price IS '加入時原價快照（新台幣元，永遠 = SKU.price，前端 strike-through 顯示用）';
COMMENT ON COLUMN cart_items.max_limit_snapshot IS '加入時限購上限快照（= product_spec.stock_qty）';
COMMENT ON COLUMN cart_items.status IS '0=NORMAL（可編輯）, 1=LOCKED（結帳鎖定中）';
