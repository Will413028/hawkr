-- platform 的 webhook inbox（Stripe 計畫 D29；單體規則 5）的 desired state。外部 provider 的事件在入口驗簽後先寫這裡，同一個
-- transaction 排入派送 job，commit 之後才回 2xx；模組的 handler 由派送 job 呼叫，在自己的 transaction 寫業務列，並經 platform 的 API
-- 標記已處理。補掃與清理是 platform 的週期 job。這裡只放表、約束與索引；改完以 atlas migrate diff 產生 migration（見 atlas.hcl）。
--
-- public 不屬於本 schema：寫在這裡只是讓 Atlas 知道它存在。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA inbox;

-- 一列是一個外部事件。(provider, event_id) 唯一：重送的事件不再寫入、不再派送。
--   - route_key 是 provider 內的路由鍵（收款是外部帳戶 ID），模組依它整批標記已處理、找出有舊的未處理事件的對象；
--   - payload 是驗過簽章的原始內容（位元組原樣），保存到 purge_after，之後由清理 job 刪除；寫 log 時不得輸出；
--   - dispatched_at 是派送 job 成功呼叫 handler 的時刻：補掃只重派還沒派送成功、也還沒處理的列（job 被丟棄或用完重試）；
--   - processed_at、processed_by 是模組標記已處理的時刻與處理者（模組的 handler 或同步 job 的名稱）。
CREATE TABLE inbox.events (
    provider      TEXT        NOT NULL,
    event_id      TEXT        NOT NULL,
    route_key     TEXT        NOT NULL,
    payload       BYTEA       NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL,
    purge_after   TIMESTAMPTZ NOT NULL,
    dispatched_at TIMESTAMPTZ,
    processed_at  TIMESTAMPTZ,
    processed_by  TEXT,
    PRIMARY KEY (provider, event_id),
    CONSTRAINT events_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT events_event_id_check CHECK (char_length(event_id) BETWEEN 1 AND 255),
    CONSTRAINT events_route_key_check CHECK (char_length(route_key) BETWEEN 1 AND 255),
    CONSTRAINT events_purge_after_check CHECK (purge_after > received_at),
    CONSTRAINT events_processed_check CHECK ((processed_at IS NULL) = (processed_by IS NULL)),
    CONSTRAINT events_processed_by_check CHECK (processed_by IS NULL OR char_length(processed_by) BETWEEN 1 AND 100)
);
CREATE INDEX events_pending ON inbox.events (provider, route_key, received_at) WHERE processed_at IS NULL;
CREATE INDEX events_undispatched ON inbox.events (received_at) WHERE processed_at IS NULL AND dispatched_at IS NULL;
CREATE INDEX events_purge ON inbox.events (purge_after) WHERE processed_at IS NOT NULL;
