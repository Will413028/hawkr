-- identity 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（identity.<表>），不建跨 schema 的 FK；慣例見 PostgreSQL 設計規範。
-- 碼值欄位是 text 加 CHECK，值等於模組的 Go 常數（ADR 2026-09-25-codemeta-retirement D3）。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA identity;

-- 帳號：登入的 principal（`docs/modules/identity.md`）。買家、賣家是同一個帳號的角色（account_roles）；業務身分
-- （buyer_id、store_id）屬於 buyer、seller 模組。
--
-- 登入由自建的聯合登入處理（ADR 2026-10-30-identity-self-hosted-federated-login）：帳號由 account_identities 的 provider 身分識別，
-- 第一次以某個身分登入時建立。刪除的帳號保留登入身分的墓碑（account_identities，同一個身分不得再建一個帳號），其他個資清掉，
-- 帳號只留墓碑，讓舊服務查得到「已刪除」。
--
-- id 過渡期是 IDENTITY：Java 服務仍把帳號 ID 存成 BIGINT，最後一個 Java 消費者改寫後以一個 migration
-- 換成 uuid（ADR 2026-09-25-row-identity-by-data-kind 的過渡規則、遷移計畫階段 C）。
--
-- 生命週期是 ACTIVE → PENDING_DELETION → DELETED：deletion_pending_at 是 accountdeletion 受理了刪除申請、等待未結訂單
-- 結束的時間（狀態由 identity 保存，流程由 accountdeletion 編排；ADR 2026-09-30-account-deletion-module 的修訂）。
-- 等待刪除的帳號不能再授予角色；刪除時清掉。
--
-- email 是 provider 回報的已驗證 email（LINE 可能沒有），登入時更新，是寄件地址，也給後台的清單顯示與搜尋。它不是識別：不唯一。
CREATE TABLE identity.accounts (
    id            BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         TEXT,
    suspended_at  TIMESTAMPTZ,
    deleted_at    TIMESTAMPTZ,
    deletion_pending_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- 刪除的帳號沒有個資，也不再是停權狀態。
    CONSTRAINT accounts_deleted_check CHECK (deleted_at IS NULL OR (email IS NULL AND suspended_at IS NULL)),
    -- 刪除的帳號不再是等待刪除。
    CONSTRAINT accounts_deletion_pending_check CHECK (deleted_at IS NULL OR deletion_pending_at IS NULL)
);

-- 帳號連結的 provider 身分（D1、D4）：(provider, subject) 是識別，同一個身分只屬於一個帳號。登入時以 provider 回報的 subject 找帳號，
-- 找不到才依規則自動連結或建帳；email 與 email_verified 是 provider 最近一次回報的值，只用於自動連結的比對與更新帳號的寄件地址，
-- 不作識別或授權。auto_link_eligible 是這個身分最近一次登入（或建立）時是否符合自動連結資格（Google、email 已驗證、網域是 gmail.com 或帶 hd）：
-- 自動連結只拿符合資格的身分的 email 當目標，因為目標的 email 也要和進來的 Google 身分一樣可信（nOAuth：不可信的 email 不能成為連結的錨點）；
-- hd 不存，只存判斷的結果。LINE 沒有給 email 時兩者是 NULL 與 false。帳號刪除時這些列留作墓碑（保留 provider 與 subject，清掉 email 與 email_verified；同一個登入身分不會再建一個帳號），登入時遇到墓碑身分被拒。
CREATE TABLE identity.account_identities (
    provider       TEXT         NOT NULL,
    subject        TEXT         NOT NULL,
    account_id     BIGINT       NOT NULL REFERENCES identity.accounts (id),
    email          TEXT,
    email_verified BOOLEAN      NOT NULL DEFAULT FALSE,
    auto_link_eligible BOOLEAN  NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ  NOT NULL,
    updated_at     TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (provider, subject),
    CONSTRAINT account_identities_provider_check CHECK (provider::text IN ('GOOGLE', 'LINE')),
    CONSTRAINT account_identities_subject_check CHECK (char_length(subject) >= 1),
    -- 已驗證一定有 email。
    CONSTRAINT account_identities_email_check CHECK (email IS NULL OR char_length(email) >= 1),
    CONSTRAINT account_identities_verified_check CHECK (NOT email_verified OR email IS NOT NULL),
    -- 符合資格一定是 Google 且 email 已驗證。
    CONSTRAINT account_identities_auto_link_check CHECK (NOT auto_link_eligible OR (provider = 'GOOGLE' AND email_verified))
);

CREATE INDEX idx_account_identities_account_id ON identity.account_identities (account_id);

-- 登入 session（opaque-sessions ADR D1–D3、自建登入 ADR D1）：token 是 32 bytes 的 CSPRNG，只存 SHA-256；id 是對外的識別
-- （列出、終止 session 用），不是憑證。閒置逾時由 last_used_at 加上 client 的期限算出（web 7 天、App 14 天，常數在 Go），
-- 絕對逾時是 absolute_expires_at（建立後 90 天）；last_used_at 一天最多寫一次。auth_time 是最近一次以 provider 驗證的時間，
-- 登入時等於 created_at，之後重新驗證時更新（終止 session 前要求 15 分鐘內驗證過）。停權、刪帳在同一個 transaction 刪該帳號全部的列。
CREATE TABLE identity.sessions (
    id                  UUID         PRIMARY KEY DEFAULT uuidv7(),
    token_hash          BYTEA        NOT NULL,
    account_id          BIGINT       NOT NULL REFERENCES identity.accounts (id),
    client              TEXT         NOT NULL,
    created_at          TIMESTAMPTZ  NOT NULL,
    last_used_at        TIMESTAMPTZ  NOT NULL,
    absolute_expires_at TIMESTAMPTZ  NOT NULL,
    auth_time           TIMESTAMPTZ  NOT NULL,
    CONSTRAINT sessions_client_check CHECK (client::text IN ('WEB', 'APP')),
    CONSTRAINT sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT sessions_times_check CHECK (last_used_at >= created_at AND absolute_expires_at > created_at AND auth_time >= created_at)
);

CREATE UNIQUE INDEX uidx_sessions_token_hash ON identity.sessions (token_hash);

CREATE INDEX idx_sessions_account_id ON identity.sessions (account_id);

-- web 登入流程的暫存狀態（自建登入 ADR D3）：start 寫一列，callback 以 state 取出並同時刪除（單次使用，驗證失敗也已作廢），
-- 10 分鐘內有效。state 與登入前 cookie 只存 SHA-256；code_verifier 與 nonce 要在換 token 與驗 ID token 時原文使用，所以存原文，
-- 列的壽命只有 10 分鐘。provider 是預期的 provider（callback 的路徑必須相同，防 mix-up）；return_path 只收 / 開頭且不是 // 或 /\
-- 的相對路徑；link_account_id 不是 NULL 時這是已登入帳號發起的手動連結。
CREATE TABLE identity.login_flows (
    state_hash       BYTEA        PRIMARY KEY,
    provider         TEXT         NOT NULL,
    code_verifier    TEXT         NOT NULL,
    nonce            TEXT         NOT NULL,
    return_path      TEXT         NOT NULL,
    binding_hash     BYTEA        NOT NULL,
    link_account_id  BIGINT       REFERENCES identity.accounts (id),
    created_at       TIMESTAMPTZ  NOT NULL,
    expires_at       TIMESTAMPTZ  NOT NULL,
    CONSTRAINT login_flows_provider_check CHECK (provider::text IN ('GOOGLE', 'LINE')),
    CONSTRAINT login_flows_state_hash_check CHECK (octet_length(state_hash) = 32),
    CONSTRAINT login_flows_binding_hash_check CHECK (octet_length(binding_hash) = 32),
    CONSTRAINT login_flows_return_path_check CHECK (return_path LIKE '/%' AND return_path NOT LIKE '//%' AND return_path NOT LIKE '/\\%'),
    CONSTRAINT login_flows_expires_check CHECK (expires_at > created_at)
);

CREATE INDEX idx_login_flows_expires_at ON identity.login_flows (expires_at);

-- App 以原生 SDK 登入前向後端要的單次 nonce（自建登入 ADR D2）：ID token 的 nonce claim 必須等於它的原文，驗證時取出並刪除，
-- 5 分鐘內有效。只存 SHA-256。
CREATE TABLE identity.app_login_nonces (
    nonce_hash  BYTEA        PRIMARY KEY,
    created_at  TIMESTAMPTZ  NOT NULL,
    expires_at  TIMESTAMPTZ  NOT NULL,
    CONSTRAINT app_login_nonces_nonce_hash_check CHECK (octet_length(nonce_hash) = 32),
    CONSTRAINT app_login_nonces_expires_check CHECK (expires_at > created_at)
);

CREATE INDEX idx_app_login_nonces_expires_at ON identity.app_login_nonces (expires_at);

-- 帳號的停權紀錄（A7-D9：處分紀錄歸執行效果的模組）：停權的理由、來源與解除在同一處，與 accounts.suspended_at 在同一個 transaction 寫入，
-- 入口只有 accountsuspension（internal/arch 的 TestSuspensionEntriesAreOnlyCalledByAccountsuspension）。解除留列不刪，之後可以再停權。
-- kind 只有 PERMANENT，沒有 ends_at（A7-D16：A7 只有永久停權與手動解除）。reason_category 是 platform/policy.Category 的全部值（模組的
-- TestCodeValueChecksMatchTheGoTypes）；source_ref 是來源（moderation 的案件）的不透明參照，identity 不讀它。同一帳號未解除的紀錄最多一筆，
-- 且若且唯若 accounts.suspended_at 不是 NULL。帳號刪除時未解除的紀錄以 release_reason = 'ACCOUNT_DELETED' 解除。
CREATE TABLE identity.account_suspensions (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    account_id      BIGINT      NOT NULL REFERENCES identity.accounts (id),
    kind            TEXT        NOT NULL DEFAULT 'PERMANENT',
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    suspended_at    TIMESTAMPTZ NOT NULL,
    released_at     TIMESTAMPTZ,
    release_reason  TEXT,
    CONSTRAINT account_suspensions_kind_check CHECK (kind::text IN ('PERMANENT')),
    CONSTRAINT account_suspensions_reason_category_check CHECK (reason_category::text IN ('FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'OTHER', 'SPAM', 'HARASSMENT', 'HATE')),
    CONSTRAINT account_suspensions_reason_text_check CHECK (char_length(reason_text) <= 1000 AND reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT account_suspensions_source_ref_check CHECK (source_ref IS NULL OR (char_length(source_ref) >= 1 AND char_length(source_ref) <= 200 AND source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT account_suspensions_release_check CHECK (
        (released_at IS NULL AND release_reason IS NULL)
        OR (released_at IS NOT NULL AND release_reason IS NOT NULL AND released_at >= suspended_at)
    ),
    CONSTRAINT account_suspensions_release_reason_check CHECK (release_reason IS NULL OR (char_length(release_reason) >= 1 AND char_length(release_reason) <= 1000 AND release_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);

CREATE UNIQUE INDEX uidx_account_suspensions_active ON identity.account_suspensions (account_id) WHERE released_at IS NULL;

CREATE INDEX idx_account_suspensions_account_id_suspended_at ON identity.account_suspensions (account_id, suspended_at DESC);

-- 帳號另外申請的角色：seller 開店時授予。買家不是角色：帳號活著就是買家（ADR 2026-09-30-account-created-provisions-buyer）。
--
-- source_id 是授予這個角色的業務身分（store_id）。舊的 seller-service 以非同步方式移除角色，可能晚於重新入駐：
-- 移除時帶的來源與目前的不同，就不移除（#3189）。A3b seller 搬進單體、在同一個 transaction 內呼叫之後拿掉。
CREATE TABLE identity.account_roles (
    account_id BIGINT       NOT NULL REFERENCES identity.accounts (id),
    role       TEXT         NOT NULL,
    source_id  BIGINT,
    granted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, role),
    CONSTRAINT account_roles_role_check CHECK (role::text IN ('SELLER'))
);

-- 管理員：與帳號是不同的 principal，由 Cloudflare Access 的白名單開通。access_subject 是 Access JWT 的 sub；
-- 第一次認證時建立。id 的過渡規則同
-- accounts（Java 以 BIGINT 記錄操作的管理員）。
--
-- email 是第一次認證時 Access JWT 回報的，只給後台顯示。
CREATE TABLE identity.admins (
    id                 BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    access_subject     TEXT         NOT NULL,
    email              TEXT         NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uidx_admins_access_subject ON identity.admins (access_subject);

-- 管理員的角色（ADR 2026-10-04-admin-roles）：誰有哪個角色；角色對權限的對應寫在程式裡（platform/adminperm），這裡只存指派。
-- 零列的管理員沒有任何權限。撤銷刪除該列，變更的歷史在 admin_role_events。
CREATE TABLE identity.admin_roles (
    admin_id   BIGINT       NOT NULL REFERENCES identity.admins (id),
    role       TEXT         NOT NULL,
    granted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (admin_id, role),
    CONSTRAINT admin_roles_role_check CHECK (role::text IN ('OWNER'))
);

-- 管理員角色的授予與撤銷歷史：只追加，與 admin_roles 的變更在同一個 transaction 內寫入，沒有 UPDATE、DELETE（queries 只有 INSERT 與 SELECT，TestAdminRoleEventsAreAppendOnly 檢查）。
-- 授予與撤銷只有 CLI 一條路，CLI 沒有經過驗證的操作者：source 記來源，db_user 是連線的 session_user、os_user 是執行者的作業系統
-- 使用者（取不到時是 NULL），兩者都未經驗證、只供參考，不提供自填操作者的參數。reason 必填，規則同 platform/auditreason。
CREATE TABLE identity.admin_role_events (
    id          UUID         PRIMARY KEY DEFAULT uuidv7(),
    admin_id    BIGINT       NOT NULL REFERENCES identity.admins (id),
    role        TEXT         NOT NULL,
    action      TEXT         NOT NULL,
    reason      TEXT         NOT NULL,
    source      TEXT         NOT NULL,
    db_user     TEXT         NOT NULL,
    os_user     TEXT,
    occurred_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_role_events_role_check CHECK (role::text IN ('OWNER')),
    CONSTRAINT admin_role_events_action_check CHECK (action::text IN ('GRANT', 'REVOKE')),
    CONSTRAINT admin_role_events_reason_check CHECK (char_length(reason) >= 1 AND char_length(reason) <= 1000 AND reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT admin_role_events_source_check CHECK (source::text IN ('CLI'))
);

CREATE INDEX idx_admin_role_events_admin_id_occurred_at ON identity.admin_role_events (admin_id, occurred_at DESC);
