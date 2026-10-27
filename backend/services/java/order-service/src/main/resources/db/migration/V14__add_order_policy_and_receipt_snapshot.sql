-- V14：訂單政策快照六欄 ＋ 收貨確認（E 軸）＋ 費基缺口欄
--      （65 號物流履約平台 Shared Foundation）
--
-- 目的：訂單成立後，不因賣家後續修改物流／收款設定而改變既有訂單的履約、收款或結算語意。
--
-- expand-contract：新欄一律「先帶 DEFAULT 加上去 → 回填 → 加 CHECK」，**不得**一步
-- NOT NULL 無 DEFAULT——否則 rolling 期間的舊 pod（不知道這些欄位存在）INSERT 會全數 23502 失敗。
-- DEFAULT 是過渡手段不是終態，收縮條件見本檔尾的 REMOVE-WHEN。

-- ─────────────────────────────────────────────────────────────
-- ① 政策快照六欄
--
-- 🔴 回填的正確性論證（不是「先填個值再說」）：
--    - prod 現有 0 筆受託代銷訂單；
--    - 既有全部訂單的收款主體是賣家子商店（orders.mer_id 來自 sellerMeta.payuni_mer_id）⇒ SELLER_MERCHANT；
--    - 既有全部訂單的買家發票由賣家開（58 號現況）⇒ SELLER_TO_BUYER。
--    ⇒ 下面四個 DEFAULT 對存量列**語意正確**，這是「可由既有事實推導」的回填，不是猜測。
--
-- ⚠️ 第 5 個維度（provider）**不在六欄內**：orders.shipping_method（V1 起就有）已經承載它。
--    再開一欄 provider 就是製造第二個真相。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE orders ADD COLUMN commerce_model   VARCHAR(24) NOT NULL DEFAULT 'SELLER_DIRECT';
ALTER TABLE orders ADD COLUMN collection_model VARCHAR(32) NOT NULL DEFAULT 'SELLER_MERCHANT';
ALTER TABLE orders ADD COLUMN fulfillment_mode VARCHAR(24) NOT NULL DEFAULT 'SELLER_MANAGED';
ALTER TABLE orders ADD COLUMN invoice_model    VARCHAR(24) NOT NULL DEFAULT 'SELLER_TO_BUYER';
ALTER TABLE orders ADD COLUMN policy_version   VARCHAR(32) NOT NULL DEFAULT 'v0-legacy';
ALTER TABLE orders ADD COLUMN consignment_agreement_version VARCHAR(32);

-- 🔴 fulfillment_mode 的存量回填：DEFAULT 'SELLER_MANAGED' 是給 rolling 期間舊 pod 用的保守值，
--    但它對存量列的語意不準——既有訂單走的是黑貓／超商／Uber，那是**平台安排的物流通道**。
--    3005（商家自行配送）今天一筆都沒有（該碼本波才新增），所以下面這句對存量是安全的。
--    不回填的後果不是報錯，是「歷史訂單在報表上全部顯示成賣家自送」——一個安靜的錯誤語意。
UPDATE orders SET fulfillment_mode = 'PLATFORM_LOGISTICS'
WHERE shipping_method IS NOT NULL AND shipping_method <> '3005';

ALTER TABLE orders ADD CONSTRAINT chk_orders_commerce_model
    CHECK (commerce_model IN ('SELLER_DIRECT', 'PLATFORM_CONSIGNMENT'));
ALTER TABLE orders ADD CONSTRAINT chk_orders_collection_model
    CHECK (collection_model IN ('SELLER_MERCHANT', 'PLATFORM_AGENT_MERCHANT', 'PROVIDER_COLLECTED', 'EXTERNAL_PREPAID'));
ALTER TABLE orders ADD CONSTRAINT chk_orders_fulfillment_mode
    CHECK (fulfillment_mode IN ('SELLER_MANAGED', 'PLATFORM_LOGISTICS'));
ALTER TABLE orders ADD CONSTRAINT chk_orders_invoice_model
    CHECK (invoice_model IN ('SELLER_TO_BUYER', 'PLATFORM_TO_BUYER'));

-- 一條雙條件式同時擋掉政策矩陣的第 3 列（本案不採用）與第 4 列（禁止：收款與開票證據不一致）
ALTER TABLE orders ADD CONSTRAINT chk_orders_policy_matrix
    CHECK ((commerce_model = 'PLATFORM_CONSIGNMENT') = (collection_model = 'PLATFORM_AGENT_MERCHANT'));

-- 開票 issuer 必須與收款主體一致
ALTER TABLE orders ADD CONSTRAINT chk_orders_invoice_collection
    CHECK ((invoice_model = 'PLATFORM_TO_BUYER') = (collection_model = 'PLATFORM_AGENT_MERCHANT'));

-- 受託代銷訂單必須帶附約版本
ALTER TABLE orders ADD CONSTRAINT chk_orders_consignment_agreement
    CHECK (commerce_model <> 'PLATFORM_CONSIGNMENT' OR consignment_agreement_version IS NOT NULL);

-- fulfillment_mode 與 shipping_method 不完全正交：3005（商家自送）必然是 SELLER_MANAGED。
-- ⚠️ 用 IS DISTINCT FROM 不是 <>：orders.shipping_method 可為 NULL，而 NULL <> '3005' 求值為 NULL
--    ⇒ CHECK 放行。那不是錯，但述詞的意圖是「只有 3005 受限」，寫 IS DISTINCT FROM 才讓它
--    對 NULL 明確為真、不依賴 CHECK 的 NULL 放行慣例。
ALTER TABLE orders ADD CONSTRAINT chk_orders_fulfillment_mode_consistent
    CHECK (shipping_method IS DISTINCT FROM '3005' OR fulfillment_mode = 'SELLER_MANAGED');

-- ─────────────────────────────────────────────────────────────
-- ② E 軸（收貨確認）：獨立 status 欄，confirmed_at／confirm_type 降為附屬事實欄
-- ─────────────────────────────────────────────────────────────
ALTER TABLE orders ADD COLUMN receipt_status            VARCHAR(24);
ALTER TABLE orders ADD COLUMN receipt_status_entered_at TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN receipt_review_due_at     TIMESTAMPTZ;

-- 回填：既有事實可完整推導出 E 軸（confirm_type 是 V1 起就有的欄，MANUAL/AUTO 兩值）
UPDATE orders SET
    receipt_status = CASE
        WHEN confirm_type = 'MANUAL'  THEN 'BUYER_CONFIRMED'
        WHEN confirm_type = 'AUTO'    THEN 'AUTO_CONFIRMED'
        WHEN delivered_at IS NOT NULL THEN 'PENDING'
        ELSE NULL END,
    -- 🔴 時戳的條件必須與上面 status 的條件**逐字對應**，否則 pair CHECK 兩邊都會炸。
    --
    --    chk_orders_receipt_entered_pair 是 `(status IS NULL) = (時戳 IS NULL)`——
    --    它有**兩種**違反方式，而兩種都真的發生過：
    --      ① status 有值、時戳 NULL：「有 confirm_type 但兩個時戳都 NULL」的列
    --      ② status NULL、時戳有值：對 status 為 NULL 的列也套了兜底值
    --    本地實測（真實 rebuild，不是 SQL 模擬）：寫成 COALESCE(..., created_at) 之後
    --    112 筆 status 為 NULL 的列全部落入 ②，Flyway 中止、order-service restart 10 次。
    --    ⚠️ 我當時的 SQL 模擬只查了 ①，所以回「0 筆違反」——**模擬錯了方向比沒模擬更危險**，
    --    它給了一個看起來已經驗過的假保證。
    --
    --    正解是讓兩個 CASE 的條件一致：status 算得出值時才給時戳。
    --    created_at 只在「該給時戳但兩個真實時戳都不可考」時當下界兜底（語意不精確，
    --    但對這種列本來就沒有更好的答案，且 orders.created_at 是 NOT NULL）。
    receipt_status_entered_at = CASE
        WHEN confirm_type IN ('MANUAL', 'AUTO') OR delivered_at IS NOT NULL
            THEN COALESCE(confirmed_at, delivered_at, created_at)
        ELSE NULL END
WHERE receipt_status IS NULL;

ALTER TABLE orders ADD CONSTRAINT chk_orders_receipt_status
    CHECK (receipt_status IS NULL OR receipt_status IN
        ('PENDING', 'BUYER_CONFIRMED', 'AUTO_CONFIRMED', 'REVIEW_PENDING',
         'ADMIN_CONFIRMED', 'ADMIN_REJECTED', 'DISPUTED'));

-- 🔴 錨在「送達這個事實」而不是「當下的 orders.status」：status 之後還會往 50/60/70 走，
--    錨在 status 的 CHECK 會在後續轉移時變成假的。delivered_at 一旦寫入就不再變動。
ALTER TABLE orders ADD CONSTRAINT chk_orders_receipt_axis_present
    CHECK (delivered_at IS NULL OR receipt_status IS NOT NULL);

ALTER TABLE orders ADD CONSTRAINT chk_orders_receipt_entered_pair
    CHECK ((receipt_status IS NULL) = (receipt_status_entered_at IS NULL));

-- ─────────────────────────────────────────────────────────────
-- ③ 費基缺口：賣家負擔的商品折扣
--    費基公式逐字要求「− 賣家負擔的商品折扣」，而既有 schema 完全沒有 bearer 維度
--    （orders.discount_amount 是單一總額）⇒ 沒有這一欄，fee_base 算不出來，
--    而症狀是「算出一個看起來合理但錯的數字」，不是報錯。
--    ⚠️ 存量列 DEFAULT 0 語意是「平台全額負擔」，對既有 SELLER_DIRECT 訂單無害
--       （不進撥款軸）；收縮階段一併 DROP DEFAULT。
-- ─────────────────────────────────────────────────────────────
ALTER TABLE orders ADD COLUMN seller_borne_discount_amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD CONSTRAINT chk_orders_seller_borne_discount_non_negative
    CHECK (seller_borne_discount_amount >= 0);

-- ─────────────────────────────────────────────────────────────
-- ④ 索引：撥款資格與人工裁決排程的候選述詞
-- ─────────────────────────────────────────────────────────────
-- 撥款軸只看 PLATFORM_CONSIGNMENT ⇒ 部分索引，存量（全為 SELLER_DIRECT）不入索引
CREATE INDEX IF NOT EXISTS idx_orders_consignment_settlement
    ON orders (commerce_model, status, delivered_at)
    WHERE commerce_model = 'PLATFORM_CONSIGNMENT';

-- 人工裁決排程的候選：receipt_review_due_at 到期且尚未裁決
CREATE INDEX IF NOT EXISTS idx_orders_receipt_review_due
    ON orders (receipt_review_due_at)
    WHERE receipt_review_due_at IS NOT NULL AND receipt_status = 'REVIEW_PENDING';

COMMENT ON COLUMN orders.commerce_model IS
    '商流模式快照（付款後鎖定）。撥款資格的軸守門：非 PLATFORM_CONSIGNMENT 一律 NOT_APPLICABLE。';
COMMENT ON COLUMN orders.collection_model IS
    '收款模式快照。🔴 resolvePaymentMerId 必須由本欄驅動，不得再讀 buyer request 透傳的 consolidated。';
COMMENT ON COLUMN orders.receipt_status IS
    'E 軸（收貨確認）狀態。NULL 的語意是「這條軸還沒開始」不是一個狀態值 ⇒ aging 指標必須排除 NULL 列，'
    '否則所有未送達訂單都會被算成「卡在 receipt 軸」。';
COMMENT ON COLUMN orders.seller_borne_discount_amount IS
    '賣家負擔的商品折扣（元）。費基公式的減項。⚠️ 連帶契約：coupon-service 必須在核銷回應中回傳 bearer 拆分，'
    '否則 order-service 沒有值可寫（本波為 open item，存量與新單皆落 0）。';

-- REMOVE-WHEN（擴張—收縮的移除承諾）：
--   commerce_model / collection_model / fulfillment_mode / invoice_model / policy_version 的 DEFAULT
--   REMOVE-WHEN: 全部寫入路徑都顯式帶值（CheckoutService 建單同交易寫入）且連續 30 天無 DEFAULT 命中
--   seller_borne_discount_amount 的 DEFAULT
--   REMOVE-WHEN: coupon-service 的 bearer 拆分契約落地，order 端改為必填
