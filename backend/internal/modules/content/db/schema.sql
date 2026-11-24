-- content 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA content;

-- 首頁 banner。id 是 UUIDv7，由應用層產生、不設 DEFAULT：寫入前就產生（ADR 2026-09-25-row-identity-by-data-kind）。
CREATE TABLE content.banners (
    id                UUID         PRIMARY KEY,
    title             TEXT         NOT NULL,
    button_text       TEXT         NOT NULL DEFAULT '',
    preview_image_key TEXT         NOT NULL,
    popup_image_key   TEXT         NOT NULL,
    -- 兩張圖的 media asset ID（UUID v7）：與 key 成對，圖片必填所以不可為 NULL；寫入在同一個 transaction 內 Confirm 新資產、MarkDeleted 被換掉的資產。
    preview_image_asset_id UUID         NOT NULL,
    popup_image_asset_id   UUID         NOT NULL,
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
    -- 點擊行為一定完整（模組的 BannerAction.validate 是第一道）：POPUP 不帶值；URL 是 http(s) 網址；
    -- PRODUCT 帶 ID；POST 帶小寫的標準 UUID（儲存的是標準形式，模組寫入時已正規化，這裡讓直接寫入的列也守住）。
    CONSTRAINT banners_action_check CHECK (
        (action_type = 'POPUP' AND action_target = '')
        OR (action_type = 'URL' AND action_target ~ '^https?://')
        OR (action_type = 'PRODUCT' AND action_target <> '')
        OR (action_type = 'POST' AND action_target ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
    )
);

COMMENT ON COLUMN content.banners.version IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
COMMENT ON COLUMN content.banners.action_type IS '點擊行為（設計書 25）：POPUP、URL、PRODUCT、POST；沒有預設值';
COMMENT ON COLUMN content.banners.action_target IS '點擊行為的值：URL 的網址、PRODUCT 與 POST 的 ID；POPUP 是空字串';

-- banner 由後台維護、筆數少，全表掃描加排序就夠快，所以除了主鍵不建索引：索引要等 EXPLAIN 證明需要才加。

-- App 版本門檻：每種 App 一組（App 只發 iOS，沒有平台維度），以 app_type 為主鍵，不對外暴露 id。
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
