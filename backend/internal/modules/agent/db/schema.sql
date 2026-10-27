-- agent 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（agent.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA agent;

-- vendor portal 的邀請（ADR 2026-10-03-vendor-portal-invite-scoped-access）：每個 RFQ 收件人一張，存取範圍就是這一張邀請。
-- rfq_recipient_id 屬於 ai-service（A9 之前無法驗證它存在），prospect_id 屬於 seller，兩者都不建 FK。invited_email 是寄出
-- 時的 email 快照（驗證比對的基準，也是這個模組唯一的個資，隨邀請清理）；已去掉前後空白，比對時才不分大小寫。secret_hash 是
-- 邀請 secret 原始 32 bytes 的 SHA-256，明文只在 EnsureInvite 的回傳與信件裡。secret_delivered_at 只為了 A8 之前
-- Ensure／Mark 的兩段式。邀請到期或撤銷之前可以重複兌換。prospect_id 在 seller 合併時改掛贏家（snapshot、secret、session 不變）。
CREATE TABLE agent.vendor_portal_invites (
    invite_id           UUID PRIMARY KEY,
    rfq_recipient_id    BIGINT NOT NULL,
    prospect_id         BIGINT NOT NULL,
    invited_email       TEXT NOT NULL,
    secret_hash         BYTEA NOT NULL,
    secret_delivered_at TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    revoke_reason       TEXT,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT vendor_portal_invites_rfq_recipient_key UNIQUE (rfq_recipient_id),
    CONSTRAINT vendor_portal_invites_secret_hash_key UNIQUE (secret_hash),
    CONSTRAINT vendor_portal_invites_rfq_recipient_check CHECK (rfq_recipient_id > 0),
    CONSTRAINT vendor_portal_invites_prospect_check CHECK (prospect_id > 0),
    CONSTRAINT vendor_portal_invites_email_check CHECK (invited_email <> '' AND invited_email !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT vendor_portal_invites_secret_hash_check CHECK (octet_length(secret_hash) = 32),
    CONSTRAINT vendor_portal_invites_revoke_reason_check CHECK (revoke_reason IN ('PROSPECT_ARCHIVED')),
    CONSTRAINT vendor_portal_invites_revoked_check CHECK ((revoked_at IS NULL) = (revoke_reason IS NULL))
);
-- prospect 封存時撤銷它的邀請、合併時改掛的查詢。
CREATE INDEX vendor_portal_invites_prospect ON agent.vendor_portal_invites (prospect_id);
-- 清理 job 依到期與撤銷時間掃描。
CREATE INDEX vendor_portal_invites_expires ON agent.vendor_portal_invites (expires_at);
CREATE INDEX vendor_portal_invites_revoked ON agent.vendor_portal_invites (revoked_at) WHERE revoked_at IS NOT NULL;

-- 兌換邀請發出的 session：不透明 token（原始 32 bytes）的 SHA-256，只屬於一張邀請。verification_*（方式、驗證的 email、
-- 時間）三者同有同無；驗證的 email 與邀請的快照相同才寫得進去（由 VerifySession 的條件 UPDATE 保證）。idle 24 小時滑動、
-- 絕對 7 天，且不超過邀請的到期時間（idle 不超過絕對期限）。
CREATE TABLE agent.vendor_portal_sessions (
    session_id          UUID PRIMARY KEY,
    token_hash          BYTEA NOT NULL,
    invite_id           UUID NOT NULL REFERENCES agent.vendor_portal_invites (invite_id) ON DELETE CASCADE,
    verification_method TEXT,
    verified_email      TEXT,
    verified_at         TIMESTAMPTZ,
    idle_expires_at     TIMESTAMPTZ NOT NULL,
    absolute_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at          TIMESTAMPTZ,
    last_seen_at        TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT vendor_portal_sessions_token_hash_key UNIQUE (token_hash),
    CONSTRAINT vendor_portal_sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT vendor_portal_sessions_method_check CHECK (verification_method IN ('GOOGLE')),
    CONSTRAINT vendor_portal_sessions_verification_check CHECK (
        (verification_method IS NULL AND verified_email IS NULL AND verified_at IS NULL)
        OR (verification_method IS NOT NULL AND verified_email IS NOT NULL AND verified_at IS NOT NULL)
    ),
    CONSTRAINT vendor_portal_sessions_expiry_check CHECK (idle_expires_at <= absolute_expires_at)
);
-- 兌換時數一張邀請的有效 session（超過上限先撤銷最舊的未驗證，全部已驗證才撤銷最舊的），撤銷時找它們。
CREATE INDEX vendor_portal_sessions_active ON agent.vendor_portal_sessions (invite_id, created_at DESC) WHERE revoked_at IS NULL;
-- 清理 job 依絕對期限掃描。
CREATE INDEX vendor_portal_sessions_absolute ON agent.vendor_portal_sessions (absolute_expires_at);
