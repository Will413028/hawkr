-- delegation 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（delegation.<表>），不建跨 schema 的 FK；授權與稽核以 UUIDv7 作主鍵；session 因舊消費者仍用 BIGINT，過渡期以 IDENTITY 發號
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA delegation;

-- 管理員的代行資格；不存在即未開通。只保存 identity 的 ID，不建跨 schema FK。
CREATE TABLE delegation.admin_eligibilities (
    admin_id BIGINT PRIMARY KEY
);

-- ============================================================================
-- 代行營運（docs/modules/delegation.md §2）。代行的編排目前在 gateway（A10 搬進本模組）；本 schema 擁有
-- session、授權與稽核資料。管理員的 ID 是 BIGINT（過渡規則同 admins），店家與賣家的 ID 是 seller 的
-- BIGINT（seller 還是舊服務），跨模組不下 FK。模組內的 FK 不寫 ON DELETE：稽核列不得被連坐刪除。
--
-- 能力（capability）是封閉的九個值，沒有任何履約或金流的能力：「本次代行沒有出貨或財務操作」由 CHECK
-- 保證，不是統計出來的。去重以九個 CASE 相加（值域已由 <@ 封閉，「沒有重複」等於元素數等於相異值數；
-- CHECK 不能用子查詢）；下界用 cardinality 而不是 array_length（後者對空陣列回 NULL，CHECK 視為通過）。
-- ============================================================================

-- 一次代行的授權事實：誰、為什麼、對哪家店、拿到哪些能力、到什麼時候為止。代行寫入的授權都從這一列來。
--
-- id 過渡期是 IDENTITY：social-service 把它存成 BIGINT（scheduled_by_session_id），gateway 與 admin 以十進位數字
-- 驗證它（ADR 2026-09-25-row-identity-by-data-kind 的過渡規則，同 accounts）。最後一個以數字存它的消費者改寫後
-- （遷移計畫階段 C）以一個 migration 換成 uuid。
--
-- display_code 是給人看的代號（OPS- 加 13 碼 0-9A-Z），由模組以 CSPRNG 產生，靠 UNIQUE 保證唯一，與 id
-- 無關（ADR 2026-09-25-row-identity-by-data-kind D1）。credential_hash 與 handoff_code_hash 只存 SHA-256；
-- Create 時 credential_hash 為 NULL，Redeem 以單句 CAS 清空 handoff_code_hash 並寫入 credential_hash。
--
-- status 只有 active、revoked：到期（hard 或閒置）在讀取時推導，不落庫。expires_at 只在主動結束時縮短，
-- 不延長。last_active_at 只由代行資源的 credential 反查順延（閒置逾時）；刻意不建索引，讓那句 UPDATE
-- 維持 HOT update。revision 只在主動結束與撤銷時加一（樂觀鎖，前端以它對帳）。
-- 秒精度：前端的時間格式最多三位小數，結束時把 expires_at 壓到 now 之後，截秒後可能等於 issued_at，
-- 前端要求 expiresAt − issuedAt > 0。
CREATE TABLE delegation.delegated_operation_sessions (
    id                       BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    display_code             TEXT         NOT NULL,
    credential_hash          BYTEA,
    handoff_code_hash        BYTEA,
    handoff_expires_at       TIMESTAMPTZ,
    actor_admin_id           BIGINT       NOT NULL,
    actor_display_name       TEXT         NOT NULL,
    target_store_id          BIGINT       NOT NULL,
    target_seller_account_id BIGINT       NOT NULL,
    target_store_name        TEXT         NOT NULL,
    reason_code              TEXT         NOT NULL,
    reason_label             TEXT         NOT NULL,
    case_reference           TEXT,
    capabilities             TEXT[]       NOT NULL,
    issued_at                TIMESTAMPTZ  NOT NULL,
    expires_at               TIMESTAMPTZ  NOT NULL,
    last_active_at           TIMESTAMPTZ  NOT NULL,
    ended_at                 TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ,
    revoked_by_admin_id      BIGINT,
    revoke_reason            TEXT,
    status                   TEXT         NOT NULL,
    revision                 INTEGER      NOT NULL DEFAULT 0,
    idempotency_key          TEXT         NOT NULL,
    create_request_id        TEXT         NOT NULL,
    create_correlation_id    TEXT         NOT NULL,
    create_user_agent        TEXT,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT delegated_operation_sessions_status_check CHECK (status::text IN ('active', 'revoked')),
    CONSTRAINT delegated_operation_sessions_reason_code_check CHECK (
        reason_code::text IN ('seller_request', 'platform_maintenance', 'content_governance')
    ),
    CONSTRAINT delegated_operation_sessions_lifetime_check CHECK (
        expires_at > issued_at AND expires_at <= issued_at + INTERVAL '1800 seconds'
    ),
    CONSTRAINT delegated_operation_sessions_second_precision_check CHECK (
        issued_at = date_trunc('second', issued_at)
        AND expires_at = date_trunc('second', expires_at)
        AND last_active_at = date_trunc('second', last_active_at)
    ),
    CONSTRAINT delegated_operation_sessions_last_active_check CHECK (
        last_active_at >= issued_at AND last_active_at <= expires_at
    ),
    CONSTRAINT delegated_operation_sessions_revision_check CHECK (revision >= 0),
    CONSTRAINT delegated_operation_sessions_revoked_check CHECK ((status = 'revoked') = (revoked_at IS NOT NULL)),
    -- 撤銷的理由是產品規則（1–200 字）：沒有理由的撤銷紀錄沒有價值。
    CONSTRAINT delegated_operation_sessions_revoke_reason_check CHECK (
        revoke_reason IS NULL OR char_length(revoke_reason) BETWEEN 1 AND 200
    ),
    -- 只能單向：兌換只清 hash、留下期限。寫成對稱的等式，兌換那句 UPDATE 會被自己的 CHECK 擋下。
    CONSTRAINT delegated_operation_sessions_handoff_check CHECK (
        handoff_code_hash IS NULL OR handoff_expires_at IS NOT NULL
    ),
    -- 案件編號是產品規則：4–200 字、至少一個數字（不做格式驗證，控制字元由模組擋）。
    CONSTRAINT delegated_operation_sessions_case_reference_check CHECK (
        case_reference IS NULL
        OR (char_length(case_reference) >= 4 AND char_length(case_reference) <= 200 AND case_reference ~ '[0-9]')
    ),
    CONSTRAINT delegated_operation_sessions_display_code_check CHECK (display_code ~ '^OPS-[0-9A-Z]{13}$'),
    CONSTRAINT delegated_operation_sessions_capabilities_check CHECK (
        cardinality(capabilities) >= 1 AND cardinality(capabilities) <= 9
        AND capabilities <@ ARRAY[
              'storefront.read', 'storefront.write',
              'products.read', 'products.write', 'products.publish',
              'posts.read', 'posts.write', 'posts.publish',
              'orders.read'
            ]::text[]
        AND cardinality(capabilities) =
              (CASE WHEN 'storefront.read'  = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'storefront.write' = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.read'    = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.write'   = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.publish' = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.read'       = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.write'      = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.publish'    = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'orders.read'      = ANY (capabilities) THEN 1 ELSE 0 END)
    )
);

CREATE UNIQUE INDEX uidx_delegated_operation_sessions_actor_idempotency
    ON delegation.delegated_operation_sessions (actor_admin_id, idempotency_key);
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_credential_hash
    ON delegation.delegated_operation_sessions (credential_hash);
-- partial，而且不得加 NULLS NOT DISTINCT：已兌換與未簽發的列都是 NULL。
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_handoff_code_hash
    ON delegation.delegated_operation_sessions (handoff_code_hash) WHERE handoff_code_hash IS NOT NULL;
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_display_code
    ON delegation.delegated_operation_sessions (display_code);
-- 一位管理員同時只代行一家店；一家店同時只被一位管理員代行。到期但還沒結束的列也佔著：partial index 的述詞
-- 不能用 now()，建立新的 session 前在同一個 transaction 內先結束它們。
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_active_actor
    ON delegation.delegated_operation_sessions (actor_admin_id) WHERE status = 'active' AND ended_at IS NULL;
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_active_store
    ON delegation.delegated_operation_sessions (target_store_id) WHERE status = 'active' AND ended_at IS NULL;
CREATE INDEX idx_delegated_operation_sessions_actor_created_at
    ON delegation.delegated_operation_sessions (actor_admin_id, created_at DESC);
CREATE INDEX idx_delegated_operation_sessions_store_created_at
    ON delegation.delegated_operation_sessions (target_store_id, created_at DESC);

-- 哪些店家可以被代行、依據什麼、到什麼時候、誰核准、何時撤回。沒有有效授權就不能代行（deny-by-default），
-- 每次代行請求都重新評估。
--
-- target_store_id 為 NULL 是平台政策的全域授權：營運上不建，保留只是讓判定規則有定義——店家層級的列存在
-- 就遮蔽全域列（不論有沒有效），全域列才不會靜默推翻逐店的撤回。每家店終生一列：重新開通是 UPDATE 同一列，
-- 歷史在 delegation_authorization_audit_logs。
CREATE TABLE delegation.seller_delegation_authorizations (
    id                   UUID         PRIMARY KEY,
    target_store_id      BIGINT,
    source               TEXT         NOT NULL,
    source_reference     TEXT         NOT NULL,
    granted_capabilities TEXT[]       NOT NULL,
    effective_from       TIMESTAMPTZ  NOT NULL,
    effective_until      TIMESTAMPTZ,
    approved_by_admin_id BIGINT       NOT NULL,
    approved_at          TIMESTAMPTZ  NOT NULL,
    revoked_at           TIMESTAMPTZ,
    revoked_by_admin_id  BIGINT,
    revoke_reason        TEXT,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT seller_delegation_authorizations_source_check CHECK (
        source::text IN ('contract', 'ticket', 'platform_policy')
    ),
    CONSTRAINT seller_delegation_authorizations_global_check CHECK (
        target_store_id IS NOT NULL OR source = 'platform_policy'
    ),
    -- 授權的依據編號是產品規則：4–200 字、至少一個數字（是編號，不是說明）。
    CONSTRAINT seller_delegation_authorizations_source_reference_check CHECK (
        char_length(source_reference) >= 4 AND char_length(source_reference) <= 200 AND source_reference ~ '[0-9]'
    ),
    CONSTRAINT seller_delegation_authorizations_period_check CHECK (
        effective_until IS NULL OR effective_until > effective_from
    ),
    -- 撤回的三欄同生同滅。
    CONSTRAINT seller_delegation_authorizations_revoked_check CHECK (
        (revoked_at IS NULL) = (revoked_by_admin_id IS NULL) AND (revoked_at IS NULL) = (revoke_reason IS NULL)
    ),
    CONSTRAINT seller_delegation_authorizations_revoke_reason_check CHECK (
        revoke_reason IS NULL OR char_length(revoke_reason) BETWEEN 1 AND 200
    ),
    CONSTRAINT seller_delegation_authorizations_capabilities_check CHECK (
        cardinality(granted_capabilities) >= 1 AND cardinality(granted_capabilities) <= 9
        AND granted_capabilities <@ ARRAY[
              'storefront.read', 'storefront.write',
              'products.read', 'products.write', 'products.publish',
              'posts.read', 'posts.write', 'posts.publish',
              'orders.read'
            ]::text[]
        AND cardinality(granted_capabilities) =
              (CASE WHEN 'storefront.read'  = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'storefront.write' = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.read'    = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.write'   = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.publish' = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.read'       = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.write'      = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.publish'    = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'orders.read'      = ANY (granted_capabilities) THEN 1 ELSE 0 END)
    )
);

CREATE UNIQUE INDEX uidx_seller_delegation_authorizations_store
    ON delegation.seller_delegation_authorizations (target_store_id) WHERE target_store_id IS NOT NULL;
CREATE UNIQUE INDEX uidx_seller_delegation_authorizations_global
    ON delegation.seller_delegation_authorizations ((target_store_id IS NULL)) WHERE target_store_id IS NULL;

-- 代行的每一筆寫入（與 PII 揭露）在動作之前先寫一列 PENDING，動作之後回填結果（兩階段稽核：編排還在
-- gateway，遷移計畫 A10 搬進單體後改成同一個 transaction）。PENDING 是合法的終態：「嘗試過、結果沒有確認」。
--
-- 寫入後不可變，只有確認時以 WHERE result = 'PENDING' 的 CAS 更新一次：result、resource_id、after_snapshot、
-- changed_fields、resource_version_after、biz_code、confirmed_at、notified_at。notified_at 是完成事件在確認的
-- 同一個 transaction 內排入送出的時刻（只有成功而且 notify_seller 的列有）。
--
-- action 是登錄的 19 個動作（設計書 56 §6.5.3：沒有合法值的動作寫不進稽核表）。結束摘要的履約、金流計數因此
-- 結構上恆為 0，一旦非零就是封鎖被打穿（§6.1.11）。
-- resource_id 是資源自己的 ID，以字串保存：商品、貼文、店家是數字，訂單以訂單號（例如 HK260728001）指稱
-- （§7.2.6 的 resource_id=orderNo）。
CREATE TABLE delegation.admin_operation_audit_logs (
    id                       UUID         PRIMARY KEY,
    session_id               BIGINT       NOT NULL REFERENCES delegation.delegated_operation_sessions (id),
    actor_admin_id           BIGINT       NOT NULL,
    actor_display_name       TEXT         NOT NULL,
    target_store_id          BIGINT       NOT NULL,
    target_seller_account_id BIGINT       NOT NULL,
    target_store_name        TEXT         NOT NULL,
    origin                   TEXT         NOT NULL,
    capability               TEXT         NOT NULL,
    action                   TEXT         NOT NULL,
    resource_type            TEXT         NOT NULL,
    resource_id              TEXT,
    resource_name            TEXT,
    result                   TEXT         NOT NULL,
    biz_code                 INTEGER,
    operation_reason         TEXT,
    case_reference           TEXT,
    request_id               TEXT         NOT NULL,
    correlation_id           TEXT         NOT NULL,
    idempotency_key          TEXT,
    payload_hash             TEXT,
    before_snapshot          JSONB,
    after_snapshot           JSONB,
    changed_fields           JSONB,
    resource_version_before  BIGINT,
    resource_version_after   BIGINT,
    user_agent               TEXT,
    risk_level               TEXT         NOT NULL,
    notify_seller            BOOLEAN      NOT NULL,
    notified_at              TIMESTAMPTZ,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    confirmed_at             TIMESTAMPTZ,
    CONSTRAINT admin_operation_audit_logs_result_check CHECK (result::text IN ('PENDING', 'SUCCEEDED', 'FAILED')),
    CONSTRAINT admin_operation_audit_logs_confirmed_check CHECK ((result = 'PENDING') = (confirmed_at IS NULL)),
    CONSTRAINT admin_operation_audit_logs_biz_code_check CHECK (result = 'FAILED' OR biz_code IS NULL),
    CONSTRAINT admin_operation_audit_logs_notified_check CHECK (
        notified_at IS NULL OR (notify_seller AND result = 'SUCCEEDED')
    ),
    CONSTRAINT admin_operation_audit_logs_risk_level_check CHECK (risk_level::text IN ('low', 'medium', 'high')),
    CONSTRAINT admin_operation_audit_logs_resource_type_check CHECK (
        resource_type::text IN ('storefront', 'product', 'post', 'order')
    ),
    CONSTRAINT admin_operation_audit_logs_origin_check CHECK (origin::text IN ('interactive', 'scheduled')),
    -- 操作理由是產品規則（1–100 字）。
    CONSTRAINT admin_operation_audit_logs_reason_check CHECK (
        operation_reason IS NULL OR char_length(operation_reason) BETWEEN 1 AND 100
    ),
    CONSTRAINT admin_operation_audit_logs_action_check CHECK (
        action::text IN (
            'STOREFRONT_UPDATE', 'STOREFRONT_LOGO_UPDATE', 'STOREFRONT_COVER_UPDATE',
            'PRODUCT_CREATE', 'PRODUCT_UPDATE', 'PRODUCT_PUBLISH', 'PRODUCT_UNPUBLISH', 'PRODUCT_BATCH_UNPUBLISH',
            'POST_CREATE', 'POST_UPDATE', 'POST_PUBLISH', 'POST_UNPUBLISH', 'POST_SCHEDULE',
            'ORDER_PII_REVEAL', 'STOREFRONT_READ', 'PRODUCT_READ', 'POST_READ', 'ORDER_READ', 'AUDIT_CORRECTION'
        )
    ),
    -- 授權層的機械兜底：稽核 intent 是每個代行操作的前置，封閉值域外的能力連 intent 都寫不進去。
    CONSTRAINT admin_operation_audit_logs_capability_check CHECK (
        capability::text IN (
            'storefront.read', 'storefront.write',
            'products.read', 'products.write', 'products.publish',
            'posts.read', 'posts.write', 'posts.publish',
            'orders.read'
        )
    )
);

CREATE INDEX idx_admin_operation_audit_logs_session_created_at
    ON delegation.admin_operation_audit_logs (session_id, created_at DESC);
CREATE INDEX idx_admin_operation_audit_logs_pending
    ON delegation.admin_operation_audit_logs (created_at) WHERE result = 'PENDING';
-- partial：PII 揭露列（沒有冪等鍵、刻意不去重）是最密集的一群。
CREATE UNIQUE INDEX uidx_admin_operation_audit_logs_idempotency
    ON delegation.admin_operation_audit_logs (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_admin_operation_audit_logs_store_created_at
    ON delegation.admin_operation_audit_logs (target_store_id, created_at DESC);
CREATE INDEX idx_admin_operation_audit_logs_actor_created_at
    ON delegation.admin_operation_audit_logs (actor_admin_id, created_at DESC);

-- 授權列的治理歷史：誰、何時、依據什麼、把範圍從什麼改成什麼。只追加，沒有 UPDATE、DELETE，也沒有
-- updated_at。與授權列在同一個 transaction 內寫入；同時是三個寫入動作的冪等紀錄（同一列授權上，同一把鍵
-- 只能成功一次）。capabilities_before 為 NULL 表示這列授權之前不存在（只有 GRANT）。before、after 刻意不加
-- 能力的封閉 CHECK：歷史列不能因為日後改了能力的值域而變得寫不進去。
CREATE TABLE delegation.delegation_authorization_audit_logs (
    id                     UUID         PRIMARY KEY,
    authorization_id       UUID         NOT NULL REFERENCES delegation.seller_delegation_authorizations (id),
    target_store_id        BIGINT       NOT NULL,
    action                 TEXT         NOT NULL,
    actor_admin_id         BIGINT       NOT NULL,
    actor_display_name     TEXT         NOT NULL,
    operation_reason       TEXT         NOT NULL,
    source                 TEXT,
    source_reference       TEXT,
    capabilities_before    TEXT[],
    capabilities_after     TEXT[],
    effective_until_before TIMESTAMPTZ,
    effective_until_after  TIMESTAMPTZ,
    idempotency_key        TEXT,
    request_id             TEXT,
    correlation_id         TEXT,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT delegation_authorization_audit_logs_action_check CHECK (
        action::text IN ('GRANT', 'REINSTATE', 'UPDATE', 'REVOKE')
    ),
    CONSTRAINT delegation_authorization_audit_logs_reason_check CHECK (
        char_length(operation_reason) BETWEEN 1 AND 200
    )
);

CREATE INDEX idx_delegation_authorization_audit_logs_store_created_at
    ON delegation.delegation_authorization_audit_logs (target_store_id, created_at DESC);
CREATE UNIQUE INDEX uidx_delegation_authorization_audit_logs_idempotency
    ON delegation.delegation_authorization_audit_logs (authorization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
