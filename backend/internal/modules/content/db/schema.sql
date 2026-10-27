-- content 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA content;

-- 首頁 banner（設計書 25）。id 是 UUIDv7，由應用層產生、不設 DEFAULT：轉正圖片時就要用 id
-- 登記引用，所以在寫入前產生（ADR 2026-09-25-row-identity-by-data-kind）。
CREATE TABLE content.banners (
    id                UUID         PRIMARY KEY,
    title             TEXT         NOT NULL,
    button_text       TEXT         NOT NULL DEFAULT '',
    preview_image_key TEXT         NOT NULL,
    popup_image_key   TEXT         NOT NULL,
    -- 兩張圖的 media asset ID：同步 job 以它們為業務列目前的資產；與 key 成對，圖片必填所以不可為 NULL。
    preview_image_asset_id TEXT         NOT NULL,
    popup_image_asset_id   TEXT         NOT NULL,
    canvas_json       TEXT         NOT NULL DEFAULT '',
    action_type       TEXT         NOT NULL,
    action_target     TEXT         NOT NULL DEFAULT '',
    display_order     INT          NOT NULL DEFAULT 0,
    is_active         BOOLEAN      NOT NULL DEFAULT FALSE,
    start_date        TIMESTAMPTZ,
    end_date          TIMESTAMPTZ,
    version           BIGINT       NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- 排程是 [開始, 結束)：結束必須晚於開始（模組的 BannerInput.validate 是第一道）。
    CONSTRAINT banners_schedule_check CHECK (start_date IS NULL OR end_date IS NULL OR start_date < end_date),
    -- 點擊行為一定完整（模組的 BannerAction.validate 是第一道）：popup 不帶值；url 是 http(s) 網址；
    -- product、post 帶 ID。
    CONSTRAINT banners_action_check CHECK (
        (action_type = 'popup' AND action_target = '')
        OR (action_type = 'url' AND action_target ~ '^https?://')
        OR (action_type IN ('product', 'post') AND action_target <> '')
    )
);

COMMENT ON COLUMN content.banners.version IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
COMMENT ON COLUMN content.banners.action_type IS '點擊行為（設計書 25）：popup、url、product、post；沒有預設值';
COMMENT ON COLUMN content.banners.action_target IS '點擊行為的值：url 的網址、product 與 post 的 ID；popup 是空字串';

-- banner 由後台維護、筆數少，全表掃描加排序就夠快，所以除了主鍵不建索引：索引要等 EXPLAIN 證明需要才加。

-- App 版本門檻（設計書 54 §5.1）：每種 App 一組（App 只發 iOS，沒有平台維度），以 app_type 為主鍵，不對外暴露 id。
CREATE TABLE content.app_version_gates (
    app_type        TEXT         NOT NULL,
    minimum_version TEXT         NOT NULL,
    store_url       TEXT         NOT NULL DEFAULT '',
    notice_title    TEXT         NOT NULL DEFAULT '',
    notice_body     TEXT         NOT NULL DEFAULT '',
    version         BIGINT       NOT NULL DEFAULT 1,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (app_type),
    CONSTRAINT app_version_gates_app_type_check CHECK (app_type::text IN ('BUYER', 'SELLER')),
    CONSTRAINT app_version_gates_minimum_version_check CHECK (minimum_version ~ '^[0-9]+\.[0-9]+\.[0-9]+$')
);

COMMENT ON COLUMN content.app_version_gates.app_type IS 'App 類型 - AppType（BUYER、SELLER）';
COMMENT ON COLUMN content.app_version_gates.version IS '樂觀鎖的版本，對外是 etag。沒有刪除，所以不必區分同一組 key 的不同建立（ADR D4）';

-- media 引用的同步版本：業務列改變 ref 的資產時在同一個 transaction 遞增 seq，同步 job 讀到 seq 之後做事，結算時只在 seq 沒變
-- 才記下 synced_seq，updated_at 是最後一次變動；保底掃描補排 synced_seq < seq 的 ref（internal/mediaref）。ref 的列不刪除：
-- 刪掉再建會讓 seq 從頭算，同步讀到的舊 seq 可能剛好相等而誤判為已同步。
CREATE TABLE content.media_ref_sync (
    ref_type   TEXT   NOT NULL,
    ref_id     TEXT   NOT NULL,
    seq        BIGINT NOT NULL DEFAULT 1,
    synced_seq BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ref_type, ref_id)
);
-- 保底掃描只看還沒同步完的 ref。
CREATE INDEX media_ref_sync_unsynced ON content.media_ref_sync (updated_at) WHERE synced_seq < seq;

-- 被業務列換掉或終態釋放的資產：同步 job 精確解除它們的引用，寫入端拒絕把它們寫回同一個 ref。unlinked_at 是第一次解除成功的時間。
CREATE TABLE content.retired_assets (
    ref_type    TEXT        NOT NULL,
    ref_id      TEXT        NOT NULL,
    asset_id    TEXT        NOT NULL,
    retired_at  TIMESTAMPTZ NOT NULL,
    unlinked_at TIMESTAMPTZ,
    PRIMARY KEY (ref_type, ref_id, asset_id)
);
CREATE INDEX retired_assets_retired_at ON content.retired_assets (retired_at);
