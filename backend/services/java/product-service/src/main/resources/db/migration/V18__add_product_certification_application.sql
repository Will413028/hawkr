-- 商品農產品認證申請與審核（19 號設計書《農業認證驗證》§4 資料模型、§15 Migration 與存量處理）。
--
-- 核心不變式：`product.certifications` JSONB 由「反正規化的公開投影」降級為
-- **只由 APPROVED 申請重算的 approved-only 投影**；權威來源改為本檔建立的
-- product_certification_application。賣家填了什麼 ≠ 平台認可什麼。
--
-- 本檔做四件事：
--   1. 建 product_certification_application（申請與審核狀態的權威表）
--   2. 建 product_certification_review_event（append-only 稽核事件）
--   3. 把既有 product.certifications 的每一筆轉成 SUBMITTED 申請（§15.3）
--   4. 清空 product.certifications 投影（不清 ⇒ 未經審核的認證繼續公開，直接違反核心不變式）

-- -----------------------------------------------------
-- 1. 認證申請表（19 §4.1）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS product_certification_application (
    id                      BIGINT PRIMARY KEY,                         -- TSID（@TableId(type = IdType.INPUT) + TsidUtil.generate()；product-service node-id-base 200）
    product_id              BIGINT      NOT NULL,                       -- 所屬商品（FK 語意；與 product_certification_review_event 一致不建 FK 約束）
    store_id                BIGINT      NOT NULL,                       -- 賣家 store 代理鍵（擁有權欄位，與 product.store_id 對齊）
    submitted_by_account_id BIGINT,                                     -- 送出者登入 principal；LEGACY_MIGRATION 的列必為 NULL（§15.3）
    certification_type      INTEGER     NOT NULL,                       -- CertificationType value（3701–3706）
    certificate_no          VARCHAR(64) NOT NULL,                       -- 只 trim，保留前導 0，不轉數字
    status                  VARCHAR(16) NOT NULL DEFAULT 'SUBMITTED',   -- ProductCertificationStatus
    source                  VARCHAR(24) NOT NULL,                       -- ProductCertificationSource
    requires_manual_review  BOOLEAN     NOT NULL DEFAULT FALSE,         -- true ⇒ 永不自動核准（§6.3 Step 3）
    gov_verify_state        VARCHAR(16) NOT NULL DEFAULT 'PENDING',     -- ProductCertificationGovVerifyState
    gov_verify_payload      JSONB,                                      -- 政府查驗結構化結果（§8.3 GovVerifyResult 的修剪版），ADM 判斷依據
    gov_verified_at         TIMESTAMPTZ,                                -- 最後一次查驗完成時間
    version                 BIGINT      NOT NULL DEFAULT 0,             -- 樂觀鎖（@Version）；ADM 裁決必帶 expectedVersion
    submitted_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),         -- 進入 SUBMITTED 的時間，ADM 佇列排序鍵
    reviewed_at             TIMESTAMPTZ,                                -- 進入終態的時間
    reviewed_by_account_id  BIGINT,                                     -- 裁決者；自動核准為 NULL（actor 由 review_event 的 SYSTEM 標示）
    reason_code             VARCHAR(32),                                -- ProductCertificationRejectReason
    public_reason           VARCHAR(255),                               -- 會展示給賣家
    internal_note           TEXT,                                       -- 僅 ADM 可見，不得出現在任何賣家／公開 DTO
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);

-- 同一 (product, type) 最多一筆待審。
-- 🔴 與 uk_pca_active 的謂詞刻意不同（SUBMITTED vs APPROVED），才能讓「舊 APPROVED ＋ 新 SUBMITTED」
--    並存 —— 這是 §3.5「換號碼時舊標章保持公開直到新的核准」成立的前提。
CREATE UNIQUE INDEX IF NOT EXISTS uk_pca_pending
    ON product_certification_application (product_id, certification_type)
    WHERE status = 'SUBMITTED';

-- 同一 (product, type) 最多一筆有效。
CREATE UNIQUE INDEX IF NOT EXISTS uk_pca_active
    ON product_certification_application (product_id, certification_type)
    WHERE status = 'APPROVED';

CREATE INDEX IF NOT EXISTS idx_pca_queue
    ON product_certification_application (status, submitted_at);                                -- ADM 佇列
CREATE INDEX IF NOT EXISTS idx_pca_lookup
    ON product_certification_application (product_id, certification_type, certificate_no);      -- §6.3 Step 1/2/3 三次查詢
CREATE INDEX IF NOT EXISTS idx_pca_store
    ON product_certification_application (store_id);                                            -- 賣家維度查詢

-- ⚠️ ck_pca_status / ck_pca_gov_state / ck_pca_cert_type 是 CodeMeta 值域的「刻意第二份拷貝」（19 §4.1）。
--    取捨：DB 層 fail-closed 比應用層 validator 更難繞過；代價是沒有任何 gate 守著它們。
--    🔴 改動 ProductCertificationStatus / ProductCertificationGovVerifyState / CertificationType 的值域時，
--       必須在同一個 PR 內補一支 V{N+1}__*.sql 同步這三個 CHECK ——
--       generate.sh 不會碰 SQL，CodeMeta 改了而 CHECK 沒改的症狀是「新代碼寫入時 DB 直接拒絕（23514）」，
--       而錯誤訊息不會告訴你是 CodeMeta 漂移。
ALTER TABLE product_certification_application
    ADD CONSTRAINT ck_pca_status CHECK (status IN
        ('SUBMITTED','APPROVED','REJECTED','WITHDRAWN','SUPERSEDED','REVOKED')),
    ADD CONSTRAINT ck_pca_gov_state CHECK (gov_verify_state IN
        ('PENDING','VERIFIED','NOT_FOUND','EXPIRED','UNSUPPORTED','UNAVAILABLE')),
    ADD CONSTRAINT ck_pca_cert_type CHECK (certification_type BETWEEN 3701 AND 3706),
    -- 這一個不是拷貝：長度／非空是 DB 層的資料完整性，與 CodeMeta 無關
    ADD CONSTRAINT ck_pca_cert_no CHECK (certificate_no <> '' AND length(certificate_no) <= 64),
    -- 恆真的不變式交給 DB 守（同 V16/V17 對 product.version / product_sku.version 的處理）
    ADD CONSTRAINT ck_pca_version_nonneg CHECK (version >= 0);

-- updated_at 安全網（與 product / product_sku / hashtag 等既有表一致）；
-- 主要路徑仍是 common-core 的 MetaObjectHandler（@TableField(fill = INSERT_UPDATE)）。
DROP TRIGGER IF EXISTS update_product_certification_application_updated_at ON product_certification_application;
CREATE TRIGGER update_product_certification_application_updated_at
    BEFORE UPDATE ON product_certification_application
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE product_certification_application IS
    '商品農產品認證申請與審核（19 號設計書 §4.1）。權威來源；product.certifications 只是它的 approved-only 公開投影。';
COMMENT ON COLUMN product_certification_application.submitted_by_account_id IS
    '送出者登入 principal（賣家本人或代行 ADM）。LEGACY_MIGRATION 的列必為 NULL —— store_id 是營運鍵、account_id 是登入 principal，禁止互相頂替（CLAUDE.md §6）。';
COMMENT ON COLUMN product_certification_application.status IS
    'ProductCertificationStatus（SUBMITTED/APPROVED/REJECTED/WITHDRAWN/SUPERSEDED/REVOKED）。只有 APPROVED 進入公開投影。';
COMMENT ON COLUMN product_certification_application.requires_manual_review IS
    'true ⇒ 永不自動核准（曾被 REJECTED/REVOKED 的號碼重送時設為 true，不得因政府查驗剛好通過就繞過裁決）。';
COMMENT ON COLUMN product_certification_application.gov_verify_payload IS
    '政府查驗結構化結果（19 §8.3 GovVerifyResult 的修剪版），key 一律 camelCase；僅 ADM 可見。';
COMMENT ON COLUMN product_certification_application.internal_note IS
    '僅 ADM 可見，不得出現在任何賣家／公開 DTO。';

-- -----------------------------------------------------
-- 2. 審核事件表（19 §4.2，append-only）
-- -----------------------------------------------------
CREATE TABLE IF NOT EXISTS product_certification_review_event (
    id                  BIGINT PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,  -- 內部表 → BIGSERIAL（@TableId(type = IdType.AUTO)）
    application_id      BIGINT      NOT NULL,                   -- FK 語意（不建 FK 約束，與本服務既有慣例一致）
    action              VARCHAR(16) NOT NULL,                   -- ProductCertificationReviewAction
    from_status         VARCHAR(16),                            -- 建立時為 NULL
    to_status           VARCHAR(16) NOT NULL,
    actor_role          VARCHAR(8)  NOT NULL,                   -- SYSTEM / SELLER / ADMIN
    actor_account_id    BIGINT,                                 -- SYSTEM 時為 NULL
    certification_type  INTEGER     NOT NULL,                   -- 快照（申請列日後被改也查得到當時值）
    certificate_no      VARCHAR(64) NOT NULL,                   -- 快照
    reason_code         VARCHAR(32),                            -- 快照
    public_reason       VARCHAR(255),                           -- 快照
    internal_note       TEXT,                                   -- 快照；僅 ADM 可見
    application_version BIGINT      NOT NULL,                   -- 事件當下的 application version
    trace_id            VARCHAR(64),                            -- 由 observability context 帶入
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pcre_application
    ON product_certification_review_event (application_id, id);

COMMENT ON TABLE product_certification_review_event IS
    '商品認證審核事件（19 號設計書 §4.2）。append-only：禁止 UPDATE / DELETE。自動核准也必須寫事件（actor_role=SYSTEM, action=AUTO_APPROVE），否則查不出標章是誰放行的。';
COMMENT ON COLUMN product_certification_review_event.actor_role IS
    'SYSTEM（自動核准／系統 SUPERSEDE）／SELLER（送出、撤回）／ADMIN（人工裁決）。';

-- -----------------------------------------------------
-- 3. 存量處理（19 §15.3）：既有 JSONB → SUBMITTED 申請
-- -----------------------------------------------------
-- TSID 在純 SQL migration 內怎麼產（19 §15.3 末段）：
--   id 是 IdType.INPUT（應用層以 TsidUtil.generate() 產生），而 Flyway 執行的是純 SQL，
--   無法呼叫 Java。本次量體極小（prod 1 筆、dev 11 筆），故以 SQL 產生單調遞增且
--   **不與應用層 TSID 值域重疊**的 id：
--     ((unix_epoch_ms) << 22) + row_number()
--   對齊的是 Java 側 TsidUtil（common-core）的佈局：42-bit timestamp + 10-bit node + 12-bit sequence
--   ⇒ 低 22 位是 node ＋ sequence，故「毫秒左移 22 位」正好落在 timestamp 段。
--   🔴 非碰撞論證：f4b6a3 TsidFactory 的 timestamp 基準是 **TSID epoch 2020-01-01**，
--      應用層值 = (unix_ms − 1577836800000) << 22；本檔用的是 **raw unix epoch**，
--      兩者差 1.578e12 ms << 22 ≈ 6.6e18，本檔產出的 id 永遠落在應用層值域之上約 50 年，
--      結構上不可能碰撞。（不用 bigserial 或小整數，那才會落進未來 TSID 的低位值域。）
--   ⚠️ bigint 上限 9.223e18 ⇒ 本式在 unix ms 達 2.199e12（約西元 2039 年）時溢位。
--      屆時若仍有存量列要遷移，PostgreSQL 會直接報 bigint out of range（失敗是響的，不是靜默的）。
INSERT INTO product_certification_application
    (id, product_id, store_id, submitted_by_account_id, certification_type, certificate_no,
     status, source, requires_manual_review, gov_verify_state, submitted_at, created_at)
SELECT ((EXTRACT(EPOCH FROM NOW()) * 1000)::BIGINT << 22) + ROW_NUMBER() OVER (ORDER BY p.id, c),
       p.id,
       p.store_id,
       NULL,                                        -- 🔴 絕不可填 p.store_id：營運鍵 ≠ 登入 principal（CLAUDE.md §6）。
                                                    --    legacy 列「沒有送出者」由 source='LEGACY_MIGRATION' 表達。
       (c->>'name')::INT,
       trim(c->>'certNo'),
       'SUBMITTED',                                 -- ⚠️ 不得標 APPROVED：沒有任何審核證據。
       'LEGACY_MIGRATION',
       FALSE,
       'PENDING',
       NOW(),
       NOW()
FROM product p, jsonb_array_elements(p.certifications) c
WHERE p.certifications IS NOT NULL
  AND jsonb_array_length(p.certifications) > 0
  AND c->>'name' ~ '^370[1-6]$'                     -- 剔除中文髒值（seed 與存量都有）
  AND coalesce(trim(c->>'certNo'), '') <> ''
  AND trim(c->>'certNo') ~ '^[A-Za-z0-9_-]{1,64}$'  -- 與 gateway certNoPattern 同一值域
ON CONFLICT DO NOTHING;
-- ⚠️ ON CONFLICT DO NOTHING 不可省略：certifications 是 JSONB 陣列、沒有唯一約束，
--    同一商品同一 type 可能有兩筆 legacy 認證 ⇒ 會撞 uk_pca_pending。
--    **不指名 conflict target 是刻意的** —— DO NOTHING 允許不指名，對任何唯一約束衝突都靜默跳過
--    （指名 DO UPDATE 才強制要求 target）。

-- -----------------------------------------------------
-- 4. 清空公開投影（19 §15.3 ②）
-- -----------------------------------------------------
-- 不清 ⇒ 未經審核的認證（含上一段被剔除的非法值）繼續對外公開，直接違反核心不變式。
-- 後果誠實標註：既有商品的標章當下消失，直到 ADM 審過為止（prod 存量 1 筆，可接受）。
-- migration 建出來的列沒有 MQ 事件，且 §16.1 明令禁止掃 PENDING 的補課排程
-- ⇒ 觸發查驗的方式是 §9.3 ADM 詳情頁的「重新查驗」（人工觸發）。
UPDATE product
SET certifications = '[]'::jsonb
WHERE certifications IS NOT NULL
  AND jsonb_array_length(certifications) > 0;
