-- Create banners table
CREATE TABLE IF NOT EXISTS banners (
    id            BIGINT PRIMARY KEY,
    title         VARCHAR(255)  NOT NULL,
    image_url     TEXT          NOT NULL,
    link_url      TEXT          NOT NULL DEFAULT '',
    display_order INT           NOT NULL DEFAULT 0,
    is_active     BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_banners_is_active_display_order ON banners (is_active, display_order);
