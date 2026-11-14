-- payment 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（payment.<表>），不建跨 schema 的 FK；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA payment;

-- 收款帳戶：擁有者在金流商那邊的外部帳戶的本地快取（Stripe 計畫 D13、D19、D28）。擁有者鍵現階段是 store_id（seller 的店家；不建跨 schema 的 FK，
-- 出現第二種擁有者時再通用化）。一個擁有者一列，關店再開沿用；只存狀態與能力快照，不存聯絡信箱（D23）。
-- seller 核准的 transaction 只 INSERT「建立中」的列（external_account_id 是 NULL、status 是 NOT_CREATED；ON CONFLICT DO NOTHING，既有的列不改）
-- 並排入建立 job；之後的寫入都由本模組的 job 與管理員的指令做：建立 job 在 transaction 外分兩步建立外部帳戶（取得 ID、完成設定，D55），第一步拿到外部 ID 就填上，
-- 第二步成功才記 create_completed_at；同步 job 重讀外部帳戶後整列覆寫（沒有觀測歷史，重讀即最新）。
-- 「已建立」看 create_completed_at，不看有沒有外部 ID（D59）：有外部 ID 而 create_completed_at 是 NULL 的是半建立的列，對外仍是未建立（status 維持 NOT_CREATED，
-- 不可開通、不輪詢、不同步）。create_called_at 是第一次呼叫金流商之前寫入的時刻（NULL 表示從沒呼叫過，這種列沒有外部帳戶，清理時直接刪）；
-- 換新的冪等鍵（舊鍵超過 30 天、列出對帳沒找到）時改成新鍵第一次呼叫之前的時刻。
-- status 是中立的五個狀態，由轉帳能力的狀態與 status_details 推導；transfers_active 與 payouts_active 是外部帳戶的能力是否啟用（seller 的開通閘門讀
-- 經 PayoutAccountChanged 複製的 transfers_active；ENABLED 等於 transfers_active）。requirements 是快照的原始內容，status_reason 是導出目前狀態的金流商原因碼（只給後台；外部帳戶已在金流商關閉時是 closed，終局，不再輪詢，D61）。
-- create_failed_at 是建立重試用完的時刻（只有還沒完成建立的列才有；後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED 的開始時間，重試建立時清空）。
-- seller_read_claimed_at 是上次賣家觸發的讀取被受理的時刻（含失敗與並行的；條件式 UPDATE 搶名額），同步 job 不讀寫。
-- purge_pending_at 是擁有者的清理已要求、還沒完成的時刻，與有沒有外部 ID 無關（清理 job 呼叫 provider 之前先標；刪列之前一直在）：本地列留著（保住 store_id 與外部 ID 或冪等鍵的對應），
-- 不被建立、不進同步輪詢、不被賣家／管理員的重新整理讀，由 payment 的週期掃描重排「關閉後刪除」，關閉（沒有外部 ID 的列先以冪等鍵取得外部帳戶）成功才刪列。
-- purge_blocked_at 與 purge_blocked_reason 是清理的終局（D59、D60）：關閉被金流商拒絕、或標成等待清理後 7 天仍然關不掉或問不到金流商；列留著，不再被週期掃描重排，
-- 出現在後台待辦 PAYMENT_ACCOUNT_PURGE_STUCK，由管理員「重試關閉」（清除終局並重新計算期限）或「已在金流商處理，刪除對應」（寫 account_mapping_deletions 並刪列）。
-- create_idempotency_key 是寫入「建立中」的列時產生的隨機 UUID（v4），建立外部帳戶的每次重試都用它當冪等鍵（Stripe 建議每個邏輯請求用隨機 UUID；
-- 不用 store_id，兩個資料庫共用同一個 Stripe 帳號時才不會撞到別人的帳戶）。同一個理由，建立外部帳戶時除了 metadata.store_id 還寫 metadata.hawkr_namespace
-- （設定 STRIPE_ACCOUNT_NAMESPACE，每個環境不同，D61），以 metadata 對帳（超過冪等鍵保存期的建立、無外部 ID 的清理）要兩者都相符，不會關到另一個環境的帳戶。
CREATE TABLE payment.accounts (
    store_id               BIGINT      PRIMARY KEY,
    provider               TEXT        NOT NULL,
    external_account_id    TEXT,
    requirement_collection TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'NOT_CREATED',
    transfers_active       BOOLEAN     NOT NULL DEFAULT false,
    payouts_active         BOOLEAN     NOT NULL DEFAULT false,
    requirements           JSONB       NOT NULL DEFAULT '{}',
    status_reason          TEXT,
    fetched_at             TIMESTAMPTZ,
    create_failed_at       TIMESTAMPTZ,
    create_called_at       TIMESTAMPTZ,
    create_completed_at    TIMESTAMPTZ,
    create_idempotency_key UUID        NOT NULL DEFAULT gen_random_uuid(),
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    seller_read_claimed_at TIMESTAMPTZ,
    purge_pending_at       TIMESTAMPTZ,
    purge_blocked_at       TIMESTAMPTZ,
    purge_blocked_reason   TEXT,
    CONSTRAINT accounts_store_id_check CHECK (store_id > 0),
    CONSTRAINT accounts_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT accounts_external_id_check CHECK (external_account_id IS NULL OR char_length(external_account_id) BETWEEN 1 AND 255),
    CONSTRAINT accounts_collection_check CHECK (requirement_collection IN ('PROVIDER', 'PLATFORM')),
    CONSTRAINT accounts_status_check CHECK (status IN ('NOT_CREATED', 'UNDER_REVIEW', 'ACTION_REQUIRED', 'ENABLED', 'DISABLED')),
    CONSTRAINT accounts_created_check CHECK ((status = 'NOT_CREATED') = (create_completed_at IS NULL)),
    CONSTRAINT accounts_enabled_check CHECK ((status = 'ENABLED') = transfers_active),
    CONSTRAINT accounts_snapshot_check CHECK (create_completed_at IS NOT NULL OR (fetched_at IS NULL AND NOT payouts_active AND status_reason IS NULL)),
    CONSTRAINT accounts_requirements_check CHECK (jsonb_typeof(requirements) = 'object'),
    CONSTRAINT accounts_status_reason_check CHECK (status_reason IS NULL OR char_length(status_reason) BETWEEN 1 AND 200),
    CONSTRAINT accounts_create_failed_check CHECK (create_failed_at IS NULL OR create_completed_at IS NULL),
    CONSTRAINT accounts_completed_check CHECK (create_completed_at IS NULL OR external_account_id IS NOT NULL),
    CONSTRAINT accounts_called_check CHECK (external_account_id IS NULL OR create_called_at IS NOT NULL),
    CONSTRAINT accounts_purge_blocked_check CHECK ((purge_blocked_at IS NULL) = (purge_blocked_reason IS NULL) AND (purge_blocked_at IS NULL OR purge_pending_at IS NOT NULL)),
    CONSTRAINT accounts_purge_blocked_reason_check CHECK (purge_blocked_reason IS NULL OR char_length(purge_blocked_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX accounts_external ON payment.accounts (provider, external_account_id) WHERE external_account_id IS NOT NULL;
CREATE INDEX accounts_poll ON payment.accounts (store_id) WHERE status IN ('UNDER_REVIEW', 'ACTION_REQUIRED', 'DISABLED');
CREATE INDEX accounts_create_failed ON payment.accounts (create_failed_at) WHERE create_failed_at IS NOT NULL;
CREATE INDEX accounts_purge_pending ON payment.accounts (store_id) WHERE purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL;
CREATE INDEX accounts_purge_blocked ON payment.accounts (purge_blocked_at) WHERE purge_blocked_at IS NOT NULL;

-- 管理員「已在金流商處理，刪除對應」的紀錄（D60）：只增不改，列被刪之後仍留著。external_account_id 是刪除當下列上的外部 ID（半建立之前沒有 ID 的列是 NULL）。
-- 不建跨 schema 的 FK，admin_id 是 identity 的管理員 ID。
CREATE TABLE payment.account_mapping_deletions (
    id                  BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id            BIGINT      NOT NULL,
    provider            TEXT        NOT NULL,
    external_account_id TEXT,
    admin_id            BIGINT      NOT NULL,
    deleted_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_mapping_deletions_store_id_check CHECK (store_id > 0),
    CONSTRAINT account_mapping_deletions_admin_id_check CHECK (admin_id > 0),
    CONSTRAINT account_mapping_deletions_external_id_check CHECK (external_account_id IS NULL OR char_length(external_account_id) BETWEEN 1 AND 255)
);
CREATE INDEX account_mapping_deletions_store ON payment.account_mapping_deletions (store_id, deleted_at);
