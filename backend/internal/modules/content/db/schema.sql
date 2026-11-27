
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA content;

CREATE TABLE content.banners (
    id                UUID         PRIMARY KEY,
    title             TEXT         NOT NULL,
    button_text       TEXT         NOT NULL DEFAULT '',
    preview_image_key TEXT         NOT NULL,
    popup_image_key   TEXT         NOT NULL,
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
    CONSTRAINT banners_schedule_check CHECK (start_date IS NULL OR end_date IS NULL OR start_date < end_date),
    CONSTRAINT banners_action_check CHECK (
        (action_type = 'POPUP' AND action_target = '')
        OR (action_type = 'URL' AND action_target ~ '^https?://')
        OR (action_type = 'PRODUCT' AND action_target ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
        OR (action_type = 'POST' AND action_target ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
    )
);

COMMENT ON COLUMN content.banners.version IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
COMMENT ON COLUMN content.banners.action_type IS '點擊行為（設計書 25）：POPUP、URL、PRODUCT、POST；沒有預設值';
COMMENT ON COLUMN content.banners.action_target IS '點擊行為的值：URL 的網址、PRODUCT 與 POST 的 ID；POPUP 是空字串';


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
