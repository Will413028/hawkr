-- assistant 模組的 desired state（Atlas 與 sqlc 共用）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（assistant.<表>），不建跨 schema 的 FK。
--
-- 四張表是 ADK v2 `session.Service` 的儲存（A9-D5）：鍵是 ADK 自己的字串（app_name、user_id、session id、event id），
-- 不是本模組的實體 ID，所以用 TEXT 而不是 UUIDv7。user_id 是買家 ID 的十進位字串（買家 ID 是 BIGINT，外部鍵、不建 FK）。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA assistant;

-- 一個對話。state 只放 session 層的 key（不含 app:、user:、temp: 前綴）；app、user 層的 state 在下面兩張表。
CREATE TABLE assistant.sessions (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    id TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_name, user_id, id),
    CONSTRAINT sessions_key_check CHECK (app_name <> '' AND user_id <> '' AND id <> ''),
    CONSTRAINT sessions_state_check CHECK (jsonb_typeof(state) = 'object')
);

-- 一則已持久化的 ADK event。event 是整個 session.Event 的 JSON（ADK 公開並測試過往返的編碼），
-- 取代逐欄存放：ADK 新增欄位時不需要改 schema。timestamp 與 id 另存成欄位，供排序與篩選。
-- partial event 不寫入；StateDelta 內的 temp: key 在寫入前剝除。
CREATE TABLE assistant.events (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    event JSONB NOT NULL,
    PRIMARY KEY (app_name, user_id, session_id, id),
    FOREIGN KEY (app_name, user_id, session_id) REFERENCES assistant.sessions (app_name, user_id, id) ON DELETE CASCADE,
    CONSTRAINT events_id_check CHECK (id <> ''),
    CONSTRAINT events_event_check CHECK (jsonb_typeof(event) = 'object')
);
CREATE INDEX events_session_time ON assistant.events (app_name, user_id, session_id, timestamp, id);

-- app: 前綴的 state，同一個 app_name 的所有對話共用。
CREATE TABLE assistant.app_states (
    app_name TEXT PRIMARY KEY,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT app_states_state_check CHECK (jsonb_typeof(state) = 'object')
);

-- user: 前綴的 state，同一個 app_name 加 user_id 的所有對話共用。
CREATE TABLE assistant.user_states (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_name, user_id),
    CONSTRAINT user_states_state_check CHECK (jsonb_typeof(state) = 'object')
);
