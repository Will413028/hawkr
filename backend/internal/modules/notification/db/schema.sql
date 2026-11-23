-- notification 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（notification.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA notification;

-- 通知意圖：來源 transaction 排的一件「某事件要通知」的紀錄。idempotency_key 是事件來源身分（型別註冊表每個型別宣告它的來源），
-- UNIQUE 保證同一事件投遞兩次只有一個意圖；它只為冪等存在，保留期與收件匣同為 180 天（超過 River 重試期即無用）。
-- params 是型別的 typed 參數快照（JSON）：只存 ID 與事件事實，不存第三方的名稱或頭像。
-- occurred_at 是來源事件發生的時間（取自事件，不是 job 處理的時間）：聚合合併時以它判斷參數誰比較新。
CREATE TABLE notification.intents (
    id              UUID        PRIMARY KEY,
    type            TEXT        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    params          JSONB       NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT intents_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT intents_type_check CHECK (type ~ '^[a-z][a-z0-9_]{0,62}$'),
    CONSTRAINT intents_idempotency_key_check CHECK (char_length(idempotency_key) BETWEEN 1 AND 200)
);
CREATE INDEX intents_created ON notification.intents (created_at);

-- 收件匣列：型別＋參數快照＋聚合鍵，讀取時依請求語系組字（收件匣不是投遞渠道，這一列本身就是「已進收件匣」的紀錄）。
-- 收件人是 (account_id, audience, store_id)：BUYER 沒有 store_id，STORE 一定有。actor_account_ids 只存帳號 ID（讀取時向擁有模組解析名稱）。
-- 有聚合鍵的未讀列同一個收件人（含 store_id）只有一列（unique index，NULLS NOT DISTINCT 涵蓋 BUYER 的 NULL store_id），後來的意圖合併進去（A8-1b 的 upsert）；已讀之後的新意圖另開一列。
-- 保留期以 updated_at（最後一次合併）起算。occurred_at 是目前參數所屬事件的發生時間（合併時只有較新的事件才換參數）。
-- 哪些意圖已經寫進這一列由 inbox_item_intents 記（worker 重試的冪等依據）。
CREATE TABLE notification.inbox_items (
    id                UUID        PRIMARY KEY,
    account_id        BIGINT      NOT NULL,
    audience          TEXT        NOT NULL,
    store_id          BIGINT,
    type              TEXT        NOT NULL,
    params            JSONB       NOT NULL,
    aggregation_key   TEXT,
    actor_account_ids BIGINT[]    NOT NULL DEFAULT '{}',
    occurred_at       TIMESTAMPTZ NOT NULL,
    read_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT inbox_items_account_check CHECK (account_id > 0),
    CONSTRAINT inbox_items_audience_check CHECK (audience IN ('BUYER', 'STORE')),
    CONSTRAINT inbox_items_store_check CHECK ((audience = 'STORE') = (store_id IS NOT NULL) AND (store_id IS NULL OR store_id > 0)),
    CONSTRAINT inbox_items_type_check CHECK (type ~ '^[a-z][a-z0-9_]{0,62}$'),
    CONSTRAINT inbox_items_aggregation_key_check CHECK (aggregation_key IS NULL OR char_length(aggregation_key) BETWEEN 1 AND 200),
    CONSTRAINT inbox_items_updated_check CHECK (updated_at >= created_at)
);
CREATE INDEX inbox_items_recipient ON notification.inbox_items (account_id, audience, store_id, updated_at DESC, id DESC);
CREATE INDEX inbox_items_unread ON notification.inbox_items (account_id, audience, store_id) WHERE read_at IS NULL;
CREATE UNIQUE INDEX inbox_items_unread_aggregation ON notification.inbox_items (account_id, audience, store_id, aggregation_key) NULLS NOT DISTINCT WHERE aggregation_key IS NOT NULL AND read_at IS NULL;
CREATE INDEX inbox_items_updated ON notification.inbox_items (updated_at);

-- 意圖與收件匣列的連結：一個意圖（只有一個收件人）寫進或合併進哪一列。intent_id 是主鍵，worker 重試時以它判斷這個意圖已經處理過，
-- 即使它先前合併進的列之後被讀過、不再是聚合目標也不會重複建列。意圖被清除時只刪連結（收件匣列留著），收件匣列被清除時連結跟著刪。
CREATE TABLE notification.inbox_item_intents (
    intent_id     UUID PRIMARY KEY REFERENCES notification.intents (id) ON DELETE CASCADE,
    inbox_item_id UUID NOT NULL REFERENCES notification.inbox_items (id) ON DELETE CASCADE
);
CREATE INDEX inbox_item_intents_item ON notification.inbox_item_intents (inbox_item_id);

-- 每渠道投遞紀錄：只記推播與 email，是每筆觸達與否的唯一來源。UNIQUE (intent, 收件人, 渠道) 是投遞冪等鍵；收件人含 store_id（一個帳號可以先後擁有已關閉的店與新店，兩間店各是一個收件人；BUYER 的 store_id 是 NULL，NULLS NOT DISTINCT 讓它也受唯一鍵約束）。
-- 送出之後 locale、sent_title、sent_body 是「實際送出的文字」（送出時的事實，保留期內可由個資清除處理，所以清除後可為 NULL）；
-- 還沒送出或永久失敗的列沒有送出文字。保留期以 created_at 起算。
CREATE TABLE notification.deliveries (
    id             UUID        PRIMARY KEY,
    intent_id      UUID        NOT NULL REFERENCES notification.intents (id) ON DELETE CASCADE,
    account_id     BIGINT      NOT NULL,
    audience       TEXT        NOT NULL,
    store_id       BIGINT,
    channel        TEXT        NOT NULL,
    status         TEXT        NOT NULL,
    locale         TEXT,
    sent_title     TEXT,
    sent_body      TEXT,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    sent_at        TIMESTAMPTZ,
    CONSTRAINT deliveries_recipient_channel_key UNIQUE NULLS NOT DISTINCT (intent_id, account_id, audience, store_id, channel),
    CONSTRAINT deliveries_account_check CHECK (account_id > 0),
    CONSTRAINT deliveries_audience_check CHECK (audience IN ('BUYER', 'STORE')),
    CONSTRAINT deliveries_store_check CHECK ((audience = 'STORE') = (store_id IS NOT NULL) AND (store_id IS NULL OR store_id > 0)),
    CONSTRAINT deliveries_channel_check CHECK (channel IN ('PUSH', 'EMAIL')),
    CONSTRAINT deliveries_status_check CHECK (status IN ('PENDING', 'SENT', 'FAILED_PERMANENT')),
    CONSTRAINT deliveries_locale_check CHECK (locale IS NULL OR locale ~ '^[a-z]{2,3}(-[A-Z]{2})?$'),
    CONSTRAINT deliveries_sent_check CHECK ((status = 'SENT') = (sent_at IS NOT NULL)),
    CONSTRAINT deliveries_sent_text_check CHECK (status = 'SENT' OR (locale IS NULL AND sent_title IS NULL AND sent_body IS NULL)),
    CONSTRAINT deliveries_failure_check CHECK ((status = 'FAILED_PERMANENT') = (failure_reason IS NOT NULL))
);
CREATE INDEX deliveries_recipient ON notification.deliveries (account_id, audience, store_id);
CREATE INDEX deliveries_status ON notification.deliveries (status, created_at) WHERE status <> 'SENT';
CREATE INDEX deliveries_created ON notification.deliveries (created_at);

-- 裝置 token：token 是主鍵，同一個 token 同時只綁一個帳號與一支 App（登入時 upsert 改綁）；沒有 platform（兩支 App 只有 iOS）。
-- app 決定推播目標：BUYER 受眾只推 BUYER App，STORE 受眾只推 SELLER App。last_registered_at 是 App 每次啟動註冊的時間，超過 60 天未更新的由 A8-3 的 job 清除。
CREATE TABLE notification.device_tokens (
    token              TEXT        PRIMARY KEY,
    account_id         BIGINT      NOT NULL,
    app                TEXT        NOT NULL,
    last_registered_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT device_tokens_token_check CHECK (char_length(token) BETWEEN 1 AND 4096),
    CONSTRAINT device_tokens_account_check CHECK (account_id > 0),
    CONSTRAINT device_tokens_app_check CHECK (app IN ('BUYER', 'SELLER'))
);
CREATE INDEX device_tokens_account ON notification.device_tokens (account_id, app);
CREATE INDEX device_tokens_last_registered ON notification.device_tokens (last_registered_at);

-- 偏好覆寫列：只存與註冊表預設不同的值（沒有列就是預設）。category 與 channel 的合法值與「不可關」規則在型別註冊表（Go），
-- 這裡只擋格式，因為分類隨型別增加（偏好 API 只接受註冊表列出的值）。
CREATE TABLE notification.preference_overrides (
    account_id BIGINT      NOT NULL,
    audience   TEXT        NOT NULL,
    category   TEXT        NOT NULL,
    channel    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (account_id, audience, category, channel),
    CONSTRAINT preference_overrides_account_check CHECK (account_id > 0),
    CONSTRAINT preference_overrides_audience_check CHECK (audience IN ('BUYER', 'STORE')),
    CONSTRAINT preference_overrides_category_check CHECK (category ~ '^[A-Z][A-Z0-9_]{0,62}$'),
    CONSTRAINT preference_overrides_channel_check CHECK (channel IN ('INBOX', 'PUSH', 'EMAIL'))
);
