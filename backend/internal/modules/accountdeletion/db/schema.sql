-- accountdeletion 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（accountdeletion.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA accountdeletion;

-- 刪帳申請是帳號層級的流程紀錄；同一帳號可有歷史申請，但同時只能有一筆待訂單完成。
CREATE TABLE accountdeletion.deletion_requests (
    id UUID PRIMARY KEY,
    account_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    CONSTRAINT deletion_requests_account_check CHECK (account_id > 0),
    CONSTRAINT deletion_requests_status_check CHECK (status IN ('PENDING_ORDERS', 'COMPLETED')),
    CONSTRAINT deletion_requests_completed_check CHECK ((status = 'COMPLETED') = (completed_at IS NOT NULL))
);
CREATE UNIQUE INDEX deletion_requests_one_pending ON accountdeletion.deletion_requests (account_id) WHERE status = 'PENDING_ORDERS';

-- 每次成功受理的 Request 都留一筆不可變的申請紀錄；重送與管理員接續本人申請也不覆寫舊紀錄。
CREATE TABLE accountdeletion.request_attempts (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES accountdeletion.deletion_requests (id),
    requester TEXT NOT NULL,
    operator_admin_id BIGINT,
    reason TEXT NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT request_attempts_requester_check CHECK (requester IN ('ACCOUNT_OWNER', 'ADMIN')),
    CONSTRAINT request_attempts_operator_check CHECK ((requester = 'ADMIN') = (operator_admin_id IS NOT NULL) AND (operator_admin_id IS NULL OR operator_admin_id > 0)),
    CONSTRAINT request_attempts_reason_check CHECK (char_length(reason) <= 1000 AND (requester <> 'ADMIN' OR (char_length(reason) >= 1 AND reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')))
);
CREATE INDEX request_attempts_request_time ON accountdeletion.request_attempts (request_id, attempted_at, id);
