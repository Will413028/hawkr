-- imageai 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（imageai.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA imageai;

-- 修圖風格（後台維護）。category 是 AiStyleCategory 的儲存碼（Go 型別加 CHECK）；每個用途有且只有一個預設風格（A9-D39：migration 為每個用途 seed 一筆，
-- 預設風格不可刪、不可停用、換預設只有 SetDefaultAiStyle，所以每個用途恆有預設）。thumbnail_key 與 thumbnail_asset_id（media asset ID，UUID v7）成對，沒有縮圖（seed 的預設風格）時 key 是空字串、asset ID 是 NULL；寫入在同一個 transaction 內 Confirm 新資產、MarkDeleted 被換掉或終態的資產。
CREATE TABLE imageai.styles (
    id                 UUID        PRIMARY KEY,
    name               TEXT        NOT NULL,
    label              TEXT        NOT NULL,
    prompt             TEXT        NOT NULL,
    category           TEXT        NOT NULL,
    thumbnail_key      TEXT        NOT NULL DEFAULT '',
    thumbnail_asset_id UUID,
    display_order      INT         NOT NULL DEFAULT 0,
    is_default         BOOLEAN     NOT NULL DEFAULT FALSE,
    enabled            BOOLEAN     NOT NULL DEFAULT TRUE,
    version            BIGINT      NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT styles_category_check CHECK (category IN ('PRODUCT', 'POST')),
    CONSTRAINT styles_name_check CHECK (char_length(name) >= 1 AND name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT styles_label_check CHECK (char_length(label) >= 1 AND label !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT styles_prompt_check CHECK (char_length(prompt) >= 1 AND char_length(prompt) <= 500 AND prompt !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT styles_default_enabled_check CHECK (NOT is_default OR enabled),
    CONSTRAINT styles_category_name_key UNIQUE (category, name)
);
CREATE UNIQUE INDEX styles_default_per_category ON imageai.styles (category) WHERE is_default;

COMMENT ON COLUMN imageai.styles.version IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
COMMENT ON COLUMN imageai.styles.prompt IS '提示詞，最多 500 個字元（A9-D40，以 Unicode 字元計）';

-- 個別設定的每週額度（後台）。沒有這一列的店用模組的預設額度（模組常數，見 backend/docs/modules/imageai.md）；weekly_limit 的上限是模組常數，寫入端檢查。
-- store_id 是外部鍵（bigint，沒有跨 schema FK）；店家退場時整列刪除（StoreRetired）。
CREATE TABLE imageai.quotas (
    store_id     BIGINT      PRIMARY KEY,
    weekly_limit INT         NOT NULL,
    memo         TEXT        NOT NULL DEFAULT '',
    version      BIGINT      NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT quotas_store_check CHECK (store_id > 0),
    CONSTRAINT quotas_weekly_limit_check CHECK (weekly_limit >= 0),
    CONSTRAINT quotas_memo_check CHECK (memo !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);

COMMENT ON COLUMN imageai.quotas.version IS '樂觀鎖的版本，對外是 etag；每次修改加一';

-- 每家店一列的修圖計數器：period_start 是計數所屬那週的週一 00:00 Asia/Taipei 對應的 UTC 時刻，進入新的一週時扣額度的 UPSERT 把 used 重設
-- （A9-D9：一列計數器加條件 UPDATE，沒有預約表；列數不隨週數成長）。
CREATE TABLE imageai.usage (
    store_id     BIGINT      PRIMARY KEY,
    period_start TIMESTAMPTZ NOT NULL,
    used         INT         NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT usage_store_check CHECK (store_id > 0),
    CONSTRAINT usage_used_check CHECK (used >= 0)
);
