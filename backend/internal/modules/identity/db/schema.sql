-- identity 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（identity.<表>），不建跨 schema 的 FK；慣例見 PostgreSQL 設計規範。
-- 碼值欄位是 text 加 CHECK，值等於模組的 Go 常數（ADR 2026-09-25-codemeta-retirement D3）。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA identity;

-- 帳號：登入的 principal（設計書 44）。買家、賣家是同一個帳號的角色（account_roles）；業務身分
-- （buyer_id、store_id）屬於 buyer、seller 模組。
--
-- 登入由 Clerk 處理（ADR 2026-09-27-identity-clerk）：clerk_user_id 是顧客的 Clerk application 的使用者 ID，
-- 第一個認證過的請求時建立帳號。刪除的帳號保留 clerk_user_id（Clerk 的使用者可能還沒刪掉，同一個 ID 不得再建一個
-- 帳號），其他個資清掉，只留墓碑，讓舊服務查得到「已刪除」。
--
-- id 過渡期是 IDENTITY：Java 服務仍把帳號 ID 存成 BIGINT，最後一個 Java 消費者改寫後以一個 migration
-- 換成 uuid（ADR 2026-09-25-row-identity-by-data-kind 的過渡規則、遷移計畫階段 C）。
--
-- 生命週期是 ACTIVE → PENDING_DELETION → DELETED：deletion_pending_at 是 accountdeletion 受理了刪除申請、等待未結訂單
-- 結束的時間（狀態由 identity 保存，流程由 accountdeletion 編排；ADR 2026-09-30-account-deletion-module 的修訂）。
-- 等待刪除的帳號不能再授予角色；刪除時清掉。
--
-- email 是建立帳號時 Clerk 回報的已驗證 email（LINE 可能沒有），只給後台的清單顯示與搜尋。它不是識別：不唯一，
-- 之後在 Clerk 改了 email 也不同步。
CREATE TABLE identity.accounts (
    id            BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    clerk_user_id TEXT         NOT NULL,
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

CREATE UNIQUE INDEX uidx_accounts_clerk_user_id ON identity.accounts (clerk_user_id);

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

-- 帳號的 Sign in with Apple refresh token（App 原生登入時交出 authorization code，背景換得）：刪除帳號的最後一步
-- 撤銷它（App Store 審核準則 5.1.1(v)）。token 是 AES-GCM 的密文，每個帳號最多一筆，重新連結時取代；撤銷的 job 排入時刪列。
CREATE TABLE identity.apple_tokens (
    account_id BIGINT       PRIMARY KEY REFERENCES identity.accounts (id),
    client_id  TEXT         NOT NULL,
    token      BYTEA        NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT apple_tokens_client_id_check CHECK (client_id <> '')
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
