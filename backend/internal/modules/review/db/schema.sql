-- review 模組的 desired state（Atlas）。改完在本目錄執行 `atlas migrate diff <名稱> --env dev`
-- 產生 migration，再在 backend/ 跑 `make sqlc`；兩者都讀這個檔。這裡只放表、約束與索引：view、函式與複合型別放 db/objects.sql
-- （Atlas 社群版在 desired state 裡遇到 view 就報錯；手寫 migration、sqlc.yaml 的 schema 加 objects.sql，見單體規則 6）。
--
-- public 不屬於本模組：寫在這裡只是讓 Atlas 知道它存在，否則 Atlas 會產生 DROP SCHEMA public。
-- 表一律寫完整名稱（review.<表>），不建跨 schema 的 FK；實體的主鍵是應用層產生的 UUIDv7、不設 DEFAULT
-- （ADR 2026-09-25-row-identity-by-data-kind）；慣例見 PostgreSQL 設計規範。

CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA review;

-- 買家對一個訂單項目的評價（一個 order_item_id 至多一則，UNIQUE 是「已評價過」的最終保證）。評價不可編輯、沒有作者自刪的入口，所以沒有 deleted_at；
-- 買家刪帳時評價被實刪（purge job 隨刪帳訂閱者另外接）。ID 是 UUIDv7；buyer_id、store_id、product_id、order_item_id 是外部的業務鍵。
-- order_no、sku_name、item_image_key、price 是建立當下取自訂單項目的快照（不收 client 的值），之後不變。content 空就是 NULL。
CREATE TABLE review.reviews (
    id             UUID        PRIMARY KEY,
    order_no       TEXT        NOT NULL,
    order_item_id  BIGINT      NOT NULL,
    buyer_id       BIGINT      NOT NULL,
    store_id       BIGINT      NOT NULL,
    product_id     BIGINT      NOT NULL,
    rating         SMALLINT    NOT NULL,
    content        TEXT,
    sku_name       TEXT        NOT NULL,
    item_image_key TEXT,
    price          BIGINT      NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL,
    reply_content  TEXT,
    replied_at     TIMESTAMPTZ,
    CONSTRAINT reviews_order_item_key UNIQUE (order_item_id),
    CONSTRAINT reviews_order_no_check CHECK (char_length(order_no) BETWEEN 1 AND 64),
    CONSTRAINT reviews_order_item_check CHECK (order_item_id > 0),
    CONSTRAINT reviews_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT reviews_store_check CHECK (store_id > 0),
    CONSTRAINT reviews_product_check CHECK (product_id > 0),
    CONSTRAINT reviews_rating_check CHECK (rating BETWEEN 1 AND 5),
    CONSTRAINT reviews_content_check CHECK (content IS NULL OR (char_length(content) >= 1 AND char_length(content) <= 1000 AND content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT reviews_sku_name_check CHECK (char_length(sku_name) <= 255),
    CONSTRAINT reviews_item_image_key_check CHECK (item_image_key IS NULL OR item_image_key <> ''),
    CONSTRAINT reviews_price_check CHECK (price >= 0),
    CONSTRAINT reviews_reply_content_check CHECK (reply_content IS NULL OR (char_length(reply_content) >= 1 AND char_length(reply_content) <= 1000 AND reply_content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT reviews_reply_pair_check CHECK ((reply_content IS NULL) = (replied_at IS NULL))
);
CREATE INDEX reviews_product ON review.reviews (product_id, created_at DESC, id DESC);
CREATE INDEX reviews_store ON review.reviews (store_id, created_at DESC, id DESC);
CREATE INDEX reviews_buyer ON review.reviews (buyer_id, created_at DESC, id DESC);

-- 評價的附圖：張數上限由 position 的範圍保證（不靠截斷）。asset_id 給媒體引用同步，object_key 給顯示。
CREATE TABLE review.review_images (
    review_id  UUID     NOT NULL REFERENCES review.reviews (id) ON DELETE CASCADE,
    position   SMALLINT NOT NULL,
    asset_id   TEXT     NOT NULL,
    object_key TEXT     NOT NULL,
    PRIMARY KEY (review_id, position),
    CONSTRAINT review_images_asset_key UNIQUE (review_id, asset_id),
    CONSTRAINT review_images_position_check CHECK (position BETWEEN 0 AND 3),
    CONSTRAINT review_images_asset_check CHECK (asset_id <> ''),
    CONSTRAINT review_images_key_check CHECK (object_key <> '')
);

-- 評價的隱藏紀錄：理由、來源、還原與稽核在同一處，可以多次隱藏與還原（還原留列不刪）。只有 review 模組寫。reason_category 是 PolicyCategory 的社群子集
-- （platform/policy.CommunityAssessable，與 social.hides 相同；TestHideCategoriesMatchPolicy）；source_ref 是 moderation 的案件的不透明參照。
-- 同一則評價未還原的隱藏最多一筆（部分唯一索引）。
CREATE TABLE review.hides (
    id              UUID        PRIMARY KEY,
    review_id       UUID        NOT NULL REFERENCES review.reviews (id) ON DELETE CASCADE,
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    admin_id        BIGINT      NOT NULL,
    hidden_at       TIMESTAMPTZ NOT NULL,
    restored_at     TIMESTAMPTZ,
    restored_by     BIGINT,
    restore_reason  TEXT,
    CONSTRAINT hides_reason_category_check CHECK (reason_category IN ('MISLEADING_INFO', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT')),
    CONSTRAINT hides_reason_text_check CHECK (char_length(reason_text) <= 500 AND reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT hides_source_ref_check CHECK (source_ref IS NULL OR (char_length(source_ref) >= 1 AND char_length(source_ref) <= 200 AND source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT hides_admin_check CHECK (admin_id > 0),
    CONSTRAINT hides_restore_check CHECK (
        (restored_at IS NULL AND restored_by IS NULL AND restore_reason IS NULL)
        OR (restored_at IS NOT NULL AND restored_by > 0 AND restore_reason IS NOT NULL AND restored_at >= hidden_at)
    ),
    CONSTRAINT hides_restore_reason_check CHECK (restore_reason IS NULL OR (char_length(restore_reason) >= 1 AND char_length(restore_reason) <= 500 AND restore_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
CREATE UNIQUE INDEX hides_active_review ON review.hides (review_id) WHERE restored_at IS NULL;
CREATE INDEX hides_review ON review.hides (review_id, hidden_at);

-- media 引用的同步版本與退場資產：形狀與語意同 social（internal/mediaref）。ref 的列不刪除。
CREATE TABLE review.media_ref_sync (
    ref_type   TEXT        NOT NULL,
    ref_id     TEXT        NOT NULL,
    seq        BIGINT      NOT NULL DEFAULT 1,
    synced_seq BIGINT      NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ref_type, ref_id)
);
CREATE INDEX media_ref_sync_unsynced ON review.media_ref_sync (updated_at) WHERE synced_seq < seq;

CREATE TABLE review.retired_assets (
    ref_type    TEXT        NOT NULL,
    ref_id      TEXT        NOT NULL,
    asset_id    TEXT        NOT NULL,
    retired_at  TIMESTAMPTZ NOT NULL,
    unlinked_at TIMESTAMPTZ,
    PRIMARY KEY (ref_type, ref_id, asset_id)
);
CREATE INDEX retired_assets_retired_at ON review.retired_assets (retired_at);
