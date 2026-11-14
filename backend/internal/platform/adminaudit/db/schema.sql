-- platform 的後台動作紀錄（D62；ADR 2026-11-14-platform-admin-audit-log）的 desired state：每個後台寫入 RPC 在領域變更的同一個 transaction 內寫一筆，
-- 誰（admin_id）、做了什麼（action，是後台 RPC 的 procedure 名稱）、對誰（target_type 與 target_id）、什麼時候（occurred_at）。
-- 只增不改：套件只有 Record 與 List，沒有改寫與刪除的路徑（沒有資料庫層的不可改約束，Atlas 社群版的 desired state 不放 trigger；觸發重新評估見 ADR）。
-- 這裡只放表、約束與索引；改完以 atlas migrate diff 產生 migration（見 atlas.hcl）。不建跨 schema 的 FK：admin_id 是 identity 的管理員 ID，target 是對象在它自己模組的 ID。
--
-- public 不屬於本 schema：寫在這裡只是讓 Atlas 知道它存在。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA adminaudit;

-- detail 只放小的結構化內容（例如動作前後的狀態代碼、是否帶了確認旗標），不放 secret、個資或原始 payload；由 Record 限制大小。
-- 保存期未定（隱私政策未定）：上線前決定。
CREATE TABLE adminaudit.records (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL,
    admin_id    BIGINT      NOT NULL,
    action      TEXT        NOT NULL,
    target_type TEXT        NOT NULL,
    target_id   TEXT        NOT NULL,
    detail      JSONB       NOT NULL DEFAULT '{}',
    CONSTRAINT records_admin_id_check CHECK (admin_id > 0),
    CONSTRAINT records_action_check CHECK (char_length(action) BETWEEN 1 AND 200),
    CONSTRAINT records_target_type_check CHECK (target_type ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT records_target_id_check CHECK (char_length(target_id) BETWEEN 1 AND 255),
    CONSTRAINT records_detail_check CHECK (jsonb_typeof(detail) = 'object' AND pg_column_size(detail) <= 2048)
);
CREATE INDEX records_by_admin ON adminaudit.records (admin_id, occurred_at);
CREATE INDEX records_by_action ON adminaudit.records (action, occurred_at);
CREATE INDEX records_by_target ON adminaudit.records (target_type, target_id, occurred_at);
CREATE INDEX records_by_time ON adminaudit.records (occurred_at);
