-- V13：Uber Direct 即時配送履約資料層（66 號詳細設計書 §5 資料設計）
--
-- 🔴 版本號說明：66 §5 開頭寫「65 已預約 V11 ⇒ 本書 DDL 落 V12」，那句話已過期——
--    V11（65 Foundation）與 V12（provider_error 拆欄）都已落地，實查目前最高版號為 V12 ⇒ 本檔為 V13。
--
-- 本檔只做「資料層」：表、欄、約束、索引。零程式碼消費者是**預期狀態**——
-- W2（provider 整合）才會有寫入者與讀取者。⚠️ 這代表本檔的任何錯誤在本波是完全靜默的，
-- 唯一的守門是 UberDirectSchemaContractTest 與 review，不要指望有測試會因為欄位打錯而轉紅。
--
-- 三問（66 §5 開頭：「第三問答不出來就不要建」）逐表寫在各段的 COMMENT ON TABLE 裡。
--
-- ⚠️ 刻意不做的事：
--   1. **不建 shipping_fee_tiers 的 3003 列**（66 §5.1.10 明令）——Uber 運費是 quote 即時價，
--      不走級距查表。⚠️ 代價：CheckoutValidator 現行的 `if (fee == null) continue;` 會命中
--      ⇒ Uber 選項在 W2 把該分支改成「查 quote」之前照樣消失。這是已知且被設計接受的，
--      不是漏建（66 §5.1.10 紅字）。
--   2. **不回填 shipments.uber_status_reconcile_next_at**（66 §5.4）——它是租約欄，
--      NULL 的正確語意就是「從未被搶佔、立即可搶」。回填成 NOW() 會讓既有列沉默一個租約週期。
--   3. **client_secret 不落任何一張表**（66 §5.1.1／§8.5）——它在 k8s secret `uber-direct-credentials`。
--   4. **不建 uber_webhook_inbox**（66 §5.1.6）——共用 payuni-webhook-service 既有的 webhook_inbox。
--      🔴 該表的兩個前置（mer_id 放寬成 nullable ＋ webhook_type 註解覆蓋）屬**該服務**的 migration，
--      不在本檔；漏做的症狀是第一筆 Uber 事件寫入時 23502（not-null violation）。
--
-- 主鍵一律 TSID（BIGINT，由 TsidUtil.generate() 產生，@TableId(type = IdType.INPUT)）——
-- 與本服務既有的 shipments／seller_shipment_settings 同慣例。
-- 金額一律 BIGINT（新台幣「元」整數），禁 NUMERIC／DECIMAL／float。
-- 時間一律 TIMESTAMPTZ（值語意 UTC）。
--
-- ⚠️ packing_policy.void_factor 是**係數不是金額** ⇒ NUMERIC(4,2)，它是本檔唯一的非整數欄位。

-- =====================================================================
-- 1. uber_account —— Uber Direct 帳號（66 §5.1.1）
-- =====================================================================
-- ① 誰寫入：ADM 設定端點（A-U1）／首次上線由維運 SQL 建立
-- ② 誰讀取：UberDirectAdapter 取 token 時（client_id ＋ organization_id）；健康檢查（66 §12.3）
-- ③ 誰推走：🔴 **不刪除**。停用走 enabled=false（66 §5.1.1 逐字）。
--    用 enabled 欄而不是 env 的理由：出事那一刻要能由 ADM **即時**關掉，而 env 改動要重新部署。
CREATE TABLE IF NOT EXISTS uber_account (
    id              BIGINT       PRIMARY KEY,
    env             VARCHAR(20)  NOT NULL,
    client_id       VARCHAR(128) NOT NULL,
    organization_id VARCHAR(128) NOT NULL,
    enabled         BOOLEAN      NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_uber_account_updated_at ON uber_account;
CREATE TRIGGER update_uber_account_updated_at
    BEFORE UPDATE ON uber_account
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE uber_account IS
    'Uber Direct 帳號（66 §5.1.1）。寫入＝ADM 設定端點／維運 SQL；讀取＝UberDirectAdapter 取 token 與健康檢查；'
    '推走＝不刪除，停用一律 enabled=false（要能被 ADM 即時關掉，env 改動要等 rollout）。只會有 1～2 列。';
COMMENT ON COLUMN uber_account.env IS
    '環境別。⚠️ 66 未定義此欄的值域 ⇒ 本檔**不加 CHECK**（加了等於我方發明一個 66 沒有的碼表）。'
    '實際值由 UBER_BASE_URL／UBER_AUTH_URL 對應的環境決定（66 §8.5）。';
COMMENT ON COLUMN uber_account.client_id IS
    'OAuth2 client_credentials 的 client_id。🔴 client_secret **不在本表**——它在 k8s secret '
    'uber-direct-credentials 的 client-secret（66 §5.1.1／§8.5）。';
COMMENT ON COLUMN uber_account.organization_id IS 'Uber 組織 id（centralized billing 的計費主體，U-H）。';
COMMENT ON COLUMN uber_account.enabled IS
    '🔴 這是「出事那一刻的煞車」，不是設定旗標：ADM 可即時關閉而不需重新部署。';

-- =====================================================================
-- 2. uber_organization —— 賣家 ↔ Uber 組織（66 §5.1.2）
-- =====================================================================
-- ① 誰寫入：賣家開通流程（onboarding）／ADM 設定端點
-- ② 誰讀取：建單與報價前解析賣家所屬組織
-- ③ 誰推走：🔴 賣家關店 ⇒ status='SUSPENDED'（**不刪**，帳務要追溯）（66 §5.1.2 逐字）
CREATE TABLE IF NOT EXISTS uber_organization (
    id              BIGINT       PRIMARY KEY,
    uber_account_id BIGINT       NOT NULL,
    store_id        BIGINT       NOT NULL,
    organization_id VARCHAR(128) NOT NULL,
    status          VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_uber_organization_status
        CHECK (status IN ('PENDING', 'ACTIVE', 'SUSPENDED'))
);

-- 查詢支援索引（非 66 §5.2 列舉；純 B-tree，不改變任何語意）
CREATE INDEX IF NOT EXISTS idx_uber_organization_store ON uber_organization (store_id);

DROP TRIGGER IF EXISTS update_uber_organization_updated_at ON uber_organization;
CREATE TRIGGER update_uber_organization_updated_at
    BEFORE UPDATE ON uber_organization
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE uber_organization IS
    '賣家 ↔ Uber 組織對應（66 §5.1.2）。寫入＝賣家開通流程／ADM；讀取＝報價與建單前解析組織；'
    '推走＝賣家關店改 SUSPENDED，🔴 不刪列（帳務要追溯）。';
COMMENT ON COLUMN uber_organization.store_id IS
    '🔴 **賣家業務鍵 store_id（sellers PK），不是 account_id**（66 §5.1.2 逐字；身分分離見設計書 44）。'
    '欄名必須誠實：這裡放 account_id 會讓整條賣家軸在 re-key 後指向錯的賣家。';
COMMENT ON COLUMN uber_organization.status IS 'PENDING｜ACTIVE｜SUSPENDED（66 §5.1.2 逐字，由 CHECK 守）。';

-- =====================================================================
-- 3. uber_store_mapping —— 賣家出貨門市對應（66 §5.1.3）
-- =====================================================================
-- ① 誰寫入：賣家設定出貨門市（S-U 端點）／ADM（A-U4）／建單失敗時的 N6 事後反推（customer_blocked 403）
-- ② 誰讀取：報價（Quote）與建單（Create）——🔴 兩處都要帶 external_store_id（66 §5.1.3）
-- ③ 誰推走：active=false（賣家關門市）；blocked 由 ADM 端點**或**下一次成功建單自動解除
--    ——🔴 **兩個出口都要有**，只留 ADM 的話賣家會卡到有人去按（66 §5.1.3 逐字）
CREATE TABLE IF NOT EXISTS uber_store_mapping (
    id                BIGINT       PRIMARY KEY,
    seller_store_id   BIGINT       NOT NULL,
    external_store_id VARCHAR(64)  NOT NULL,
    customer_id       VARCHAR(64)  NOT NULL,
    pickup_snapshot   JSONB        NOT NULL,
    supported_temps   JSONB        NOT NULL DEFAULT '["3201"]',
    max_packages      INTEGER      NOT NULL DEFAULT 30,
    prep_sla_minutes  INTEGER      NOT NULL DEFAULT 15,
    active            BOOLEAN      NOT NULL DEFAULT false,
    blocked           BOOLEAN      NOT NULL DEFAULT false,
    tax_form_required BOOLEAN      NOT NULL DEFAULT false,
    health_status     VARCHAR(20)  NOT NULL DEFAULT 'UNKNOWN',
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_uber_store_mapping_store_external UNIQUE (seller_store_id, external_store_id),
    CONSTRAINT chk_uber_store_mapping_health
        CHECK (health_status IN ('HEALTHY', 'DEGRADED', 'BLOCKED', 'UNKNOWN')),
    -- INV-U6（66 §5.5）：active=true ⇒ blocked=false。
    -- 違反的症狀是「對已封鎖門市持續建單，每筆 403」，而 403 在我方看起來像偶發錯誤。
    CONSTRAINT chk_uber_store_mapping_active_not_blocked
        CHECK ((NOT active) OR (NOT blocked))
);

DROP TRIGGER IF EXISTS update_uber_store_mapping_updated_at ON uber_store_mapping;
CREATE TRIGGER update_uber_store_mapping_updated_at
    BEFORE UPDATE ON uber_store_mapping
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE uber_store_mapping IS
    '賣家出貨門市 ↔ Uber external_store 對應（66 §5.1.3）。寫入＝賣家設定端點／ADM／建單 403 的 N6 反推；'
    '讀取＝Quote 與 Create 兩處；推走＝active=false（關門市），blocked 由 ADM 端點**或**下一次成功建單'
    '自動解除（🔴 兩個出口都要有，只留 ADM 的話賣家會卡到有人去按）。';
COMMENT ON COLUMN uber_store_mapping.seller_store_id IS
    '🔴 賣家業務鍵 store_id（sellers PK），不是 account_id（66 §2.4）。';
COMMENT ON COLUMN uber_store_mapping.external_store_id IS
    '🔴 Quote 與 Create 兩處都要帶（66 §5.1.3）。⚠️ 同時是**憑證等級**的值（66 §8.6：'
    '洩漏 ＋ token 洩漏 ＝ 可代賣家叫車）⇒ log 必須遮罩。';
COMMENT ON COLUMN uber_store_mapping.pickup_snapshot IS
    '🔴 **快照不是參照**：取件地址／聯絡人／電話／備註在建立當下的值。賣家改地址後，'
    '已建立的 delivery 不得跟著變，否則對帳時「這一趟從哪裡取件」會有兩個答案。'
    '⚠️ 但 quote 用的是**當下值** ⇒ payload_hash 必須含 external_store_id（66 A9）。';
COMMENT ON COLUMN uber_store_mapping.supported_temps IS
    '此門市支援的溫層（DeliveryTemp value 陣列）。🔴 預設只有常溫 ["3201"]（66 §5.1.3）——'
    'Uber API 完全沒有溫層欄位（U-M），檢查 100% 在我方。';
COMMENT ON COLUMN uber_store_mapping.max_packages IS '件數上限（M-4：30 件硬擋，無 warning 層）。';
COMMENT ON COLUMN uber_store_mapping.prep_sla_minutes IS '備貨時間，進 quote 的 pickup_ready_dt（預設 15 分）。';
COMMENT ON COLUMN uber_store_mapping.blocked IS
    'Uber customer_blocked 403 的事後反推結果（66 N6／§4.5.4）。⚠️ 它沒有主動查詢的來源，'
    '只能由一次失敗的建單反推 ⇒ 解除同樣要有自動出口，不能只靠人。';
COMMENT ON COLUMN uber_store_mapping.tax_form_required IS '同 blocked，由 customer_blocked 403 反推（賣家需完成稅務資料）。';
COMMENT ON COLUMN uber_store_mapping.health_status IS 'HEALTHY｜DEGRADED｜BLOCKED｜UNKNOWN（66 §5.1.3 逐字，由 CHECK 守）。';

-- =====================================================================
-- 4. uber_quotes —— Uber 報價（66 §5.1.4）
-- =====================================================================
-- ① 誰寫入：報價端點／checkout preview（CREATED）
-- ② 誰讀取：checkout confirm（INV-U1 的收斂點：confirm 只讀本表，**不重算運費**）／建單／客訴與帳務追溯
-- ③ 誰推走（四問「走」那一格，66 §5.1.4 逐字）：
--      CHECKOUT_ACCEPTED ← 結帳確認
--      CONSUMED          ← 建單成功。🔴 allowed-from **只允許 CHECKOUT_ACCEPTED**
--                           （from CREATED 代表跳過了 §4.1 ② 的結帳確認）
--      EXPIRED           ← TTL 掃描（idx_uber_quotes_expiry 的候選集合）
--      SUPERSEDED        ← 同 payload_hash 重報
--    卡住偵測：uber_quote_stuck_rows{status}——CHECKOUT_ACCEPTED 超過 30 分未 CONSUMED
--    ＝**收了錢沒建單**。
--    ⚠️ EXPIRED 只改 status **不刪列**（帳務與客訴要查得到報價歷史），保留 90 天。
CREATE TABLE IF NOT EXISTS uber_quotes (
    id                     BIGINT       PRIMARY KEY,
    quote_id               VARCHAR(64)  NOT NULL,
    order_no               VARCHAR(64),
    buyer_id               BIGINT       NOT NULL,
    seller_store_id        BIGINT       NOT NULL,
    external_store_id      VARCHAR(64)  NOT NULL,
    fee_amount             BIGINT       NOT NULL,
    currency               VARCHAR(8)   NOT NULL,
    expires_at             TIMESTAMPTZ  NOT NULL,
    pickup_json            JSONB        NOT NULL,
    dropoff_json           JSONB        NOT NULL,
    items_json             JSONB        NOT NULL,
    payload_hash           CHAR(64)     NOT NULL,
    packing_policy_version INTEGER      NOT NULL,
    status                 VARCHAR(24)  NOT NULL DEFAULT 'CREATED',
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uk_uber_quotes_quote_id UNIQUE (quote_id),
    CONSTRAINT chk_uber_quotes_status
        CHECK (status IN ('CREATED', 'CHECKOUT_ACCEPTED', 'EXPIRED', 'CONSUMED', 'SUPERSEDED')),
    CONSTRAINT chk_uber_quotes_fee_non_negative CHECK (fee_amount >= 0)
);

-- 66 §5.2 逐字：quote TTL 掃描的候選集合。
-- ⚠️ partial index 的 WHERE 述詞必須與 mapper 的 SQL **逐字**相符，否則 PostgreSQL 不會用它，
--    而且不會報錯——只會慢。TTL 掃描的述詞固定是 status='CREATED' AND expires_at <= NOW()。
CREATE INDEX IF NOT EXISTS idx_uber_quotes_expiry
    ON uber_quotes (expires_at)
    WHERE status = 'CREATED';

-- 查詢支援索引（非 66 §5.2 列舉；純 B-tree，不改變任何語意）
CREATE INDEX IF NOT EXISTS idx_uber_quotes_payload_hash ON uber_quotes (payload_hash);
CREATE INDEX IF NOT EXISTS idx_uber_quotes_order_no ON uber_quotes (order_no);

DROP TRIGGER IF EXISTS update_uber_quotes_updated_at ON uber_quotes;
CREATE TRIGGER update_uber_quotes_updated_at
    BEFORE UPDATE ON uber_quotes
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE uber_quotes IS
    'Uber 報價（66 §5.1.4）。寫入＝報價端點／checkout preview；讀取＝checkout confirm（INV-U1 收斂點：'
    '🔴 confirm 只讀本表不重算運費，程式裡不得存在呼叫 shippingFeeCache.lookupFee("3003", …) 的分支）／'
    '建單／帳務追溯；推走＝CHECKOUT_ACCEPTED（結帳確認）・CONSUMED（建單成功，allowed-from 只有 '
    'CHECKOUT_ACCEPTED）・EXPIRED（TTL 掃描）・SUPERSEDED（同 hash 重報）。'
    '⚠️ EXPIRED 只改 status 不刪列，保留 90 天。';
COMMENT ON COLUMN uber_quotes.quote_id IS 'Uber 回傳的報價 id（dqt_ 前綴）。UNIQUE。';
COMMENT ON COLUMN uber_quotes.order_no IS '建單後回填；報價當下為 NULL（66 §5.1.4）。';
COMMENT ON COLUMN uber_quotes.fee_amount IS
    '報價運費（新台幣**元**整數）。🔴 INV-U1：訂單 Uber 運費恆等於被 CHECKOUT_ACCEPTED 的那筆的本欄值，'
    '**永不補收**——補收＝對買家違約。';
COMMENT ON COLUMN uber_quotes.payload_hash IS
    'SHA256( external_store_id ‖ canonical(dropoff) ‖ canonical(items 依 sku_id 排序) ‖ packing_policy_version )'
    '——順序固定，缺一即 66 A9（換了取件門市但 hash 相同 ⇒ 用舊報價建新門市的單）。'
    '🔴 **由伺服端算，前端不得傳**（前端傳 hash 等於讓前端決定「這是不是同一筆購物車」）。';
COMMENT ON COLUMN uber_quotes.packing_policy_version IS
    '🔴 政策版本進 quote **快照**：改政策不回溯既有訂單，否則同一張訂單在不同時間算出不同運費，'
    '而運費已經收過了（66 §5.1.8）。';
COMMENT ON COLUMN uber_quotes.status IS
    'CREATED｜CHECKOUT_ACCEPTED｜EXPIRED｜CONSUMED｜SUPERSEDED（66 §5.1.4 逐字，由 CHECK 守）。'
    '終態＝CONSUMED／EXPIRED／SUPERSEDED；非終態＝CREATED／CHECKOUT_ACCEPTED（66 §7.3.5）。';

-- =====================================================================
-- 5. uber_deliveries —— Uber 配送單（66 §5.1.5）
-- =====================================================================
-- ① 誰寫入：建單成功時（🔴 ON CONFLICT DO NOTHING，不是 catch DuplicateKeyException——
--    PostgreSQL 交易內任一句失敗即整個交易 aborted，catch 得到例外但下一句 SELECT 會炸成 500）；
--    webhook consumer（delivery_status／courier_update）；P1 未終態回查排程
-- ② 誰讀取：賣家／買家追蹤端點、狀態映射、POD 抓取、對帳（uber_delivery_id ＋ cancelation_reason）
-- ③ 誰推走：webhook 與 P1 回查把 provider_status 推到終態（delivered／canceled／returned）並寫 outcome；
--    🔴 **列不刪除**（帳務與爭議要追溯）。終態清單見 66 §7.3.5。
CREATE TABLE IF NOT EXISTS uber_deliveries (
    id                   BIGINT       PRIMARY KEY,
    delivery_id          VARCHAR(128) NOT NULL,
    shipment_id          BIGINT       NOT NULL,
    quote_id             VARCHAR(64),
    is_return            BOOLEAN      NOT NULL DEFAULT false,
    related_deliveries   JSONB,
    provider_status      VARCHAR(32),
    courier_imminent     BOOLEAN      NOT NULL DEFAULT false,
    courier_name         VARCHAR(128),
    courier_phone_masked VARCHAR(32),
    courier_location     JSONB,
    eta_dropoff_at       TIMESTAMPTZ,
    undeliverable_reason VARCHAR(64),
    cancelation_reason   VARCHAR(64),
    outcome              VARCHAR(32),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- INV-U2（66 §5.5）：每筆 CONSUMED quote 恰有一筆 uber_deliveries。違反＝兩張單、重複計費。
    CONSTRAINT uk_uber_deliveries_quote_id UNIQUE (quote_id),
    -- INV-U3（66 §5.5）：每筆 uber_deliveries 恰有一筆 shipments。違反＝狀態寫不回去。
    CONSTRAINT uk_uber_deliveries_shipment_id UNIQUE (shipment_id),
    -- 66 §5.3：建單成功但寫入失敗時重試會拿到同一個 delivery_id ⇒ 本 UNIQUE 是最後一道。
    CONSTRAINT uk_uber_deliveries_delivery_id UNIQUE (delivery_id),
    -- Uber 原生 7 值（U-I／66 §7.3.1 逐字）。
    -- ⚠️ 本 CHECK 是 **fail-closed 的最後一道**，不是「把 Uber 值域納進 CodeMeta」——
    --    66 附錄 C.4 明令這些原生值不納管（它隨時可能被 Uber 改，且 fromValue 對未知值會拋，
    --    而我們要的是 counter 不是例外）。寫入路徑對未知值本來就 fail-closed（不寫入 ＋ counter ＋ 建 case），
    --    本 CHECK 只擋「有人繞過那條路徑」。🔴 若 Uber 真的擴充值域，改的是**新的 migration**，
    --    不是把這個 CHECK 拿掉當成「反正它也沒用」。
    --    ⚠️ 容忍值 shopping_completed（Pick & Pack 專屬）刻意不在本清單：66 §7.3.1 逐字「counter ＋ 不寫入狀態」。
    CONSTRAINT chk_uber_deliveries_provider_status
        CHECK (provider_status IS NULL OR provider_status IN
            ('pending', 'pickup', 'pickup_complete', 'dropoff', 'delivered', 'canceled', 'returned')),
    -- UberDeliveryOutcome（66 附錄 C.2 ④ 逐字五值）
    CONSTRAINT chk_uber_deliveries_outcome
        CHECK (outcome IS NULL OR outcome IN
            ('PROVIDER_UNDELIVERABLE', 'PROVIDER_CANCELED', 'SELLER_CANCELED', 'ADMIN_CANCELED', 'CREATE_FAILED'))
);

-- 查詢支援索引（非 66 §5.2 列舉；純 B-tree，不改變任何語意）
CREATE INDEX IF NOT EXISTS idx_uber_deliveries_provider_status ON uber_deliveries (provider_status);

DROP TRIGGER IF EXISTS update_uber_deliveries_updated_at ON uber_deliveries;
CREATE TRIGGER update_uber_deliveries_updated_at
    BEFORE UPDATE ON uber_deliveries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE uber_deliveries IS
    'Uber 配送單（66 §5.1.5）。寫入＝建單成功（ON CONFLICT DO NOTHING）／webhook consumer／P1 回查排程；'
    '讀取＝追蹤端點、狀態映射、POD 抓取、對帳；推走＝webhook 與 P1 把 provider_status 推到終態'
    '（delivered／canceled／returned）並寫 outcome，🔴 列不刪除。'
    '⚠️ updated_at 的最舊齡是 runbook 的 stale ETA 判準（66 §12.4）。';
COMMENT ON COLUMN uber_deliveries.delivery_id IS
    'Uber 配送單 id（del_ 正向／ret_ 逆向）。🔴 **前綴只用於讀取時的健全性檢查，不用於分辨 leg**'
    '（66 §4.5.3／陷阱 A7）——前綴是 provider 的實作細節，它改了我們不會知道。'
    '讀到「ret_ 前綴但 is_return=false」⇒ 建 case，不猜。';
COMMENT ON COLUMN uber_deliveries.is_return IS
    '🔴 **leg 的真相在這裡，不在前綴**（66 §5.1.5）。逆向 leg 是另一筆 shipment ＋ 另一筆本表列，'
    '不是在同一筆上加狀態。';
COMMENT ON COLUMN uber_deliveries.related_deliveries IS
    '正向 ↔ 逆向 leg 的關聯陣列（66 N2／§7.3.3）。🔴 relationship 未知值 fail-closed：'
    '不建關聯 ＋ 建 case，**不做前綴比對、不猜**。';
COMMENT ON COLUMN uber_deliveries.provider_status IS
    'Uber 原生狀態值照抄（7 值，U-I）。正規化值在 shipments.status，兩者分開儲存。'
    '🔴 未知值一律 fail-closed（不寫入 ＋ counter ＋ 建 case），不得猜測式映射（INV-U7）。';
COMMENT ON COLUMN uber_deliveries.courier_imminent IS
    '🔴 **boolean 欄位不是狀態**（U-I／66 §7.3.1）：它不映射任何 normalized 值。'
    '把它當第 8 個狀態會讓狀態機多出一條不存在的邊。';
COMMENT ON COLUMN uber_deliveries.courier_phone_masked IS
    '🔴 存**遮罩後**的顯示值。Uber 給的是可撥打的臨時號碼 ⇒ 原值只透傳給前端，'
    '**不落庫不落 log**（66 §5.1.5／§8.6）。';
COMMENT ON COLUMN uber_deliveries.undeliverable_reason IS
    'Uber 原生值照抄（66 附錄 C.4 不納管）。returned／canceled 時的原因快照，'
    '是 33（ABNORMAL）留痕的內容（66 §7.3.4）。';
COMMENT ON COLUMN uber_deliveries.cancelation_reason IS
    'Uber 原生值照抄。🔴 取消**可能產生費用而該費用只出現在帳單**（U-R／66 §4.5.5）⇒ '
    '本欄與 shipments.uber_delivery_id 是月結對帳的鍵；**不得為取消費用另造 shipments 欄位**（沒有來源的欄會永遠是 null）。';
COMMENT ON COLUMN uber_deliveries.outcome IS
    'UberDeliveryOutcome（66 C.2 ④）：PROVIDER_UNDELIVERABLE｜PROVIDER_CANCELED｜SELLER_CANCELED｜'
    'ADMIN_CANCELED｜CREATE_FAILED。🔴 它是「41 這一格的錢與責任歸誰」的唯一判準——'
    '41 也可能來自賣家主動取消，兩者的錢與責任完全不同（66 §7.3.4）。';

-- =====================================================================
-- 6. uber_pod_artifacts —— 送達憑證（66 §5.1.7）
-- =====================================================================
-- ① 誰寫入：delivery 進 delivered 時插入待抓列；P2 POD 抓取排程回填 asset_id／object_key／fetched_at
-- ② 誰讀取：賣家／買家的 POD 端點（S-U5）、爭議處理
-- ③ 誰推走：P2 抓到後寫 fetched_at（⇒ 離開 idx_pod_fetch_lease 的候選集合）；
--    逾 7 天窗口（T-12）未抓到 ⇒ 建 case UBER_POD_MISSING 並在 timeline 留一行「POD 不可得，原因 X」。
--    🔴 fetch_lease_until 是**租約欄不是「已抓過」旗標**——pod 在抓取中途崩潰時，租約到期會讓該筆
--    自動回到候選集合；永久旗標則會讓它卡死且無告警。
CREATE TABLE IF NOT EXISTS uber_pod_artifacts (
    id                BIGINT       PRIMARY KEY,
    delivery_id       VARCHAR(128) NOT NULL,
    artifact_type     VARCHAR(20)  NOT NULL,
    asset_id          BIGINT,
    object_key        VARCHAR(512),
    pincode_value     VARCHAR(32),
    fetched_at        TIMESTAMPTZ,
    fetch_attempts    INTEGER      NOT NULL DEFAULT 0,
    fetch_lease_until TIMESTAMPTZ,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- 66 §7.9：POD 重複上傳由本 UNIQUE 擋。
    CONSTRAINT uk_uber_pod_artifacts_delivery_type UNIQUE (delivery_id, artifact_type),
    -- Uber 原生值照抄（66 §5.1.7 逐字三值）
    CONSTRAINT chk_uber_pod_artifacts_type
        CHECK (artifact_type IN ('picture', 'signature', 'pincode')),
    CONSTRAINT chk_uber_pod_artifacts_attempts CHECK (fetch_attempts >= 0)
);

-- 66 §5.2 逐字：POD 抓取候選。
-- ⚠️ WHERE 述詞必須與 mapper 的 claim SQL 逐字相符（fetched_at IS NULL）。
CREATE INDEX IF NOT EXISTS idx_pod_fetch_lease
    ON uber_pod_artifacts (fetch_lease_until)
    WHERE fetched_at IS NULL;

DROP TRIGGER IF EXISTS update_uber_pod_artifacts_updated_at ON uber_pod_artifacts;
CREATE TRIGGER update_uber_pod_artifacts_updated_at
    BEFORE UPDATE ON uber_pod_artifacts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE uber_pod_artifacts IS
    'Uber 送達憑證 POD（66 §5.1.7／§7.8）。寫入＝delivered 時插入待抓列 ＋ P2 排程回填；'
    '讀取＝賣家／買家 POD 端點與爭議處理；推走＝P2 抓到後寫 fetched_at（離開候選集合），'
    '逾 7 天（T-12）建 case UBER_POD_MISSING。'
    '🔴 INV-U5 是「每筆 delivered 在 7 天內有 ≥1 筆」而不是「三種都要有」——'
    'leave_at_door 只有 picture，要求齊全會讓每一單都被判成缺證據。';
COMMENT ON COLUMN uber_pod_artifacts.artifact_type IS
    'picture｜signature｜pincode（Uber 原生值照抄，66 §5.1.7）。'
    'leave_at_door ⇒ 只有 picture；需簽收 ⇒ signature（可能另有 picture）；需 PIN ⇒ pincode。';
COMMENT ON COLUMN uber_pod_artifacts.object_key IS
    '🔴 欄名叫 _key、存的就是 key（不是 URL）——secure/ 前綴（66 §5.1.7／§25 多媒體規範）。'
    '⚠️ 用 public/ 前綴的後果是陷阱 A6：收件地址與門口實景可被任何拿到 objectKey 的人取得。'
    '⚠️ objectKey 本身是**憑證等級**的值（66 §8.6）⇒ log 必須遮罩。';
COMMENT ON COLUMN uber_pod_artifacts.fetch_lease_until IS
    '🔴 **租約到期時間，不是「已抓過」旗標**（66 §5.1.7）。NULL 或 <= NOW() 表示可搶佔；'
    '租約長度 10 分鐘（T-11，下界 > POD deadline 20s×3 ＋ media 20s ≈80s；上界 < 7 天窗口）。';
COMMENT ON COLUMN uber_pod_artifacts.fetch_attempts IS '抓取嘗試次數（退避與放棄判準用）。';

-- =====================================================================
-- 7. packing_policy —— 分箱政策（版本化）（66 §5.1.8）
-- =====================================================================
-- ① 誰寫入：維運／ADM 新增版本（append-only，🔴 不 UPDATE 既有版本）
-- ② 誰讀取：報價時的分箱演算法；歷史訂單以 uber_quotes.packing_policy_version 回溯當時政策
-- ③ 誰推走：🔴 **不推走、不刪除**——改政策是「新增一個版本」，舊版本必須永久保留，
--    否則 uber_quotes.packing_policy_version 會指向不存在的政策，
--    「這張訂單當時是用什麼規則算的」就沒有答案（66 §5.1.8：改政策不回溯既有訂單）。
--    生效版本 ＝ effective_from <= NOW() 中最大的 version。
CREATE TABLE IF NOT EXISTS packing_policy (
    id                    BIGINT       PRIMARY KEY,
    version               INTEGER      NOT NULL,
    void_factor           NUMERIC(4,2) NOT NULL DEFAULT 1.25,
    packaging_weight_gram INTEGER      NOT NULL DEFAULT 150,
    max_items_per_package INTEGER      NOT NULL DEFAULT 12,
    effective_from        TIMESTAMPTZ  NOT NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    -- 66 §5.1.8 把 version 寫成 PK；本 repo 的主鍵慣例是 TSID ⇒ version 落為 UNIQUE 業務鍵，
    -- 語意完全等價（它仍是 uber_quotes.packing_policy_version 指向的那個值）。
    CONSTRAINT uk_packing_policy_version UNIQUE (version),
    -- fail-closed：void_factor <= 0 會讓「下界估計 × 係數」恆為 0 ⇒ 體積檢查恆過，而且完全無聲。
    CONSTRAINT chk_packing_policy_void_factor CHECK (void_factor > 0),
    CONSTRAINT chk_packing_policy_weight CHECK (packaging_weight_gram >= 0),
    -- max_items_per_package <= 0 會讓分箱演算法算不出任何一箱（或無窮迴圈）。
    CONSTRAINT chk_packing_policy_max_items CHECK (max_items_per_package > 0)
);

DROP TRIGGER IF EXISTS update_packing_policy_updated_at ON packing_policy;
CREATE TRIGGER update_packing_policy_updated_at
    BEFORE UPDATE ON packing_policy
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE packing_policy IS
    '分箱政策（版本化，66 §5.1.8）。寫入＝維運／ADM 新增版本（append-only）；讀取＝報價分箱演算法 ＋ '
    '歷史訂單回溯；推走＝🔴 不推走不刪除（舊版本被 uber_quotes.packing_policy_version 指著，'
    '刪了「這張訂單當時用什麼規則算的」就沒有答案）。';
COMMENT ON COLUMN packing_policy.version IS
    '政策版本號。66 §5.1.8 原文寫它是 PK；本 repo 主鍵慣例為 TSID ⇒ 這裡落為 UNIQUE 業務鍵。'
    'uber_quotes.packing_policy_version 指向本欄。';
COMMENT ON COLUMN packing_policy.void_factor IS
    '材積膨脹係數（🔴 **係數不是金額** ⇒ 唯一使用 NUMERIC 的欄位）。'
    '⚠️ 它是**估計不是事實** ⇒ 體積檢查是「下界估計 × 係數」⇒ 失敗文案必須是「超過限制」'
    '而不是「箱子放不下」（66 §5.1.8）。';
COMMENT ON COLUMN packing_policy.packaging_weight_gram IS '每箱包材重量（公克），計入 package_plan.weight_gram。';
COMMENT ON COLUMN packing_policy.max_items_per_package IS '單箱最大件數。';
COMMENT ON COLUMN packing_policy.effective_from IS '本版本生效時間（UTC）。生效版本＝effective_from <= NOW() 中 version 最大者。';

-- 初始政策版本（66 §5.1.8 逐字的三個預設值）。
-- ⚠️ 沒有這一列，uber_quotes.packing_policy_version 在 W2 第一次報價時就沒有可指向的版本。
-- 本列是**配置**不是測試資料（同 V1 對 shipping_method_configs 的初始資料），且在 uber.enabled=false
-- 之前完全惰性。
-- ⚠️ id 硬寫 1 而非 TSID：本列由 migration 建立，沒有 application 可以呼叫 TsidUtil.generate()；
--    1 與 TSID 的值域（毫秒時間戳 << 22）不可能相撞，且固定值讓這一列在各環境可對帳。
INSERT INTO packing_policy (id, version, void_factor, packaging_weight_gram, max_items_per_package, effective_from)
VALUES (1, 1, 1.25, 150, 12, NOW())
ON CONFLICT DO NOTHING;

-- =====================================================================
-- 8. package_plan —— 分箱結果（66 §5.1.8）
-- =====================================================================
-- ① 誰寫入：報價時的分箱演算法（與 uber_quotes 同一次寫入）
-- ② 誰讀取：建單時組 Uber packages 欄位；承載檢查（§7.4 道 3／道 4／道 5）；客訴追溯
-- ③ 誰推走：🔴 生命週期**掛在 quote 上**——quote 保留 90 天（66 §5.1.4），本表的列隨該 quote
--    一同由既有的資料清理排程（DataCleanupScheduler，66 §0.5）移除。列本身沒有狀態要推。
CREATE TABLE IF NOT EXISTS package_plan (
    id          BIGINT      PRIMARY KEY,
    quote_id    VARCHAR(64) NOT NULL,
    seq         INTEGER     NOT NULL,
    length_cm   INTEGER     NOT NULL,
    width_cm    INTEGER     NOT NULL,
    height_cm   INTEGER     NOT NULL,
    weight_gram INTEGER     NOT NULL,
    size_class  VARCHAR(20),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- 同一筆 quote 的箱號不得重複：報價重試時靠它擋出重複的箱子（否則件數檢查會看到雙倍箱數）。
    CONSTRAINT uk_package_plan_quote_seq UNIQUE (quote_id, seq),
    -- fail-closed：任一邊為 0 會讓體積（L×W×H）恆為 0 ⇒ §7.4 道 3 恆過，而且完全無聲。
    CONSTRAINT chk_package_plan_dimensions
        CHECK (length_cm > 0 AND width_cm > 0 AND height_cm > 0),
    CONSTRAINT chk_package_plan_weight CHECK (weight_gram > 0)
);

DROP TRIGGER IF EXISTS update_package_plan_updated_at ON package_plan;
CREATE TRIGGER update_package_plan_updated_at
    BEFORE UPDATE ON package_plan
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE package_plan IS
    '分箱結果（66 §5.1.8）。寫入＝報價時的分箱演算法（與 uber_quotes 同一次寫入）；'
    '讀取＝建單組 packages ＋ 承載檢查（§7.4 道 3／4／5）；'
    '推走＝生命週期掛在 quote 上，隨 quote 的 90 天保留期由既有資料清理排程移除。';
COMMENT ON COLUMN package_plan.quote_id IS '對應 uber_quotes.quote_id（dqt_ 前綴的那個值，不是本服務的 TSID）。';
COMMENT ON COLUMN package_plan.seq IS '本 quote 內的箱號（1 起）。';
COMMENT ON COLUMN package_plan.weight_gram IS '本箱重量（公克，🔴 **含包材** packing_policy.packaging_weight_gram）。';
COMMENT ON COLUMN package_plan.size_class IS
    'Uber 的 size（🔴 **string enum 不是向量**，U-K／66 N4）。'
    '⚠️ 66 未列出其值域 ⇒ 本欄**不加 CHECK**（加了等於我方發明一個 66 沒有的碼表）；'
    '值域由 W2 的 sandbox golden fixture 覆核後再決定要不要釘。';

-- =====================================================================
-- 9. 既有表擴充：shipments（66 §5.1.10）
-- =====================================================================
-- 🔴 **不得回填**（66 §5.4 逐字）：它是**租約欄**，NULL 的正確語意就是「從未被搶佔、立即可搶」。
--    回填成 NOW() 會讓既有列沉默一個租約週期（V10 檔頭已把這條寫成先例）。
ALTER TABLE shipments ADD COLUMN IF NOT EXISTS uber_status_reconcile_next_at TIMESTAMPTZ;

COMMENT ON COLUMN shipments.uber_status_reconcile_next_at IS
    '66 P1：Uber 未終態回查（T-8 tick 5 分／T-9 租約 5 分）的**租約到期時間**，'
    '🔴 不是「已回查旗標」。NULL 或 <= NOW() 表示可搶佔。'
    '⚠️ P1 不是 reconciliation job（66 §12.5）：webhook 只重試 3 次（U-D，窗口約 100 秒），'
    '回查是我方唯一的收斂手段，屬業務流程的一部分。';

-- 66 §5.2 逐字：回查候選
--   shipping_method='3003' AND status NOT IN ('11','41')
--   AND (uber_status_reconcile_next_at IS NULL OR uber_status_reconcile_next_at <= NOW())
-- ⚠️ partial index 的 WHERE 述詞必須與 mapper 的 claim SQL **逐字**相符——不符時 PostgreSQL
--    不會用這個索引，而且不會報錯（只會慢），所以沒有任何東西會告訴你它壞了。
CREATE INDEX IF NOT EXISTS idx_shipments_uber_reconcile_lease
    ON shipments (uber_status_reconcile_next_at)
    WHERE shipping_method = '3003' AND status NOT IN ('11', '41');

-- =====================================================================
-- 10. 既有表擴充：shipping_method_configs（66 §5.1.10／§3.4 ①）
-- =====================================================================
-- max_volume_cm3：repo 現況**完全沒有體積概念**——既有 isWithinLimit 只有 max_dimension_sum_cm
-- （量綱是 cm，L+W+H 相加）／max_single_side_cm／max_weight_grams 三項。
-- 🔴 不需要回填（66 §5.4 逐字）：0 = 不限，與同表既有欄位（max_dimension_sum_cm 等）語意一致。
ALTER TABLE shipping_method_configs ADD COLUMN IF NOT EXISTS max_volume_cm3 BIGINT NOT NULL DEFAULT 0;

COMMENT ON COLUMN shipping_method_configs.max_volume_cm3 IS
    '體積上限（立方公分 L×W×H），0 表示不限（沿用同表既有慣例）。'
    '⚠️ 與 max_dimension_sum_cm **量綱不同**：後者是 L+W+H 相加（cm），本欄是相乘（cm³）。'
    '兩者都存在不是重複——黑貓走材積和、Uber 走體積。';

-- 3003（Uber 即時配送）／3201（常溫）設定列。
--
-- 🔴 **enabled = false 是本列存在的全部理由**（66 §3.4 ①／陷阱 A1′）：
--    「建列」與「開啟」是兩件事。列以 enabled=false 建立時是**惰性的**——CheckoutValidator
--    查不到 enabled 的列就跟現在一樣不提供 Uber 選項；真正把真實流量放進來的是 66 §12.2 步 9
--    那一次 false→true 的切換，而那一步是**整個遷移唯一不可逆的一步**。
--    ⚠️ 顛倒順序（程式還在 Phase 1 就把 enabled 切 true）＝陷阱 A1：買家付錢、建出沒人送的單、零告警。
--
-- 🔴 **本列不是測試資料**，是 production 配置（同 V1 對本表既有五列的處理）；
--    infra/seed-data/ 放的是 dev 測試 fixture（seller_shipment_settings 之類），不是這種東西。
--    ⚠️ 66 §12.2 步 1 寫「走 seed 路徑不寫進 migration」——但本 repo 沒有 production 配置的 seed 路徑
--    （infra/seed-data/06-shipping.sql 實查零 shipping_method_configs），而同表既有五列就在 V1 裡。
--    本檔沿用 V1 的既有作法；差異已回報給主對話回灌 66。
--
-- 欄位值來源：
--   max_dimension_sum_cm  = 0       → 66 未給 Uber 的 L+W+H 上限 ⇒ 0（不限），走 max_volume_cm3 那條
--   max_single_side_cm    = 60      → M-3「我方最長邊 60 cm」，66 §0.1 逐字「用既有 max_single_side_cm」
--   max_weight_grams      = 20000   → M-1「我方重量上限 20,000 g」（與黑貓 3001 同值，賣家不用學第二套）
--   max_volume_cm3        = 125000  → M-2「我方體積上限 125,000 cm³（50³）」
--   estimated_days        = 0       → 即時配送當日送達（本欄語意是 D+N 的 N）
--   settlement_days       = 0       ⚠️ 66 未指定。Uber 走月結帳單（§4.5.5 ③），撥款天期屬 65 帳本軸
--   commission_rate/min   = 0 / 0   ⚠️ 66 未指定。本欄是「PayUni 代收手續費」，Uber 不經 PayUni 代收
-- 🔴 標 ⚠️ 的三個值是本檔的暫定值，**必須在 66 §12.2 步 9（enabled 切 true）之前覆核**——
--    它們在 enabled=false 期間完全不被讀取，所以在那之前沒有任何東西會告訴你它們是錯的。
INSERT INTO shipping_method_configs (
    shipping_method, delivery_temp,
    max_dimension_sum_cm, max_single_side_cm, max_weight_grams, max_volume_cm3,
    estimated_days, settlement_days, commission_rate, commission_min,
    enabled
) VALUES (
    '3003', '3201',
    0, 60, 20000, 125000,
    0, 0, 0, 0,
    false
)
-- 不指名 conflict target：DO NOTHING 對**任何**唯一約束衝突都靜默跳過，
-- 這是同一張表同時存在新舊唯一約束時唯一安全的寫法（見 PostgreSQL 設計規範）。
ON CONFLICT DO NOTHING;

-- 🔴 **刻意不建 shipping_fee_tiers 的 3003 列**（66 §5.1.10 紅字）——
--    Uber 運費是 quote 即時價，不走級距查表。這一行註解存在的理由是：
--    下一個人看到「3003 在 configs 有列、在 fee_tiers 沒列」時會以為是漏掉的。
