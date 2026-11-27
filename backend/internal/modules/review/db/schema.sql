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
-- 買家刪帳時評價被實刪（purge job 隨刪帳訂閱者另外接）。ID 是 UUIDv7；buyer_id、store_id、order_item_id 是外部的業務鍵，product_id 是 catalog 的商品 UUID。
-- order_no、sku_name、item_image_key、price 是建立當下取自訂單項目的快照（不收 client 的值），之後不變。content 空就是 NULL。
CREATE TABLE review.reviews (
    id             UUID        PRIMARY KEY,
    order_no       TEXT        NOT NULL,
    order_item_id  BIGINT      NOT NULL,
    buyer_id       BIGINT      NOT NULL,
    store_id       BIGINT      NOT NULL,
    product_id     UUID        NOT NULL,
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
    CONSTRAINT reviews_product_check CHECK (product_id <> '00000000-0000-0000-0000-000000000000'),
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

-- 評價的附圖：張數上限由 position 的範圍保證（不靠截斷）。asset_id 是 media 的資產 ID（UUID v7，寫入時在同一個 transaction 內 Confirm），object_key 給顯示。
CREATE TABLE review.review_images (
    review_id  UUID     NOT NULL REFERENCES review.reviews (id) ON DELETE CASCADE,
    position   SMALLINT NOT NULL,
    asset_id   UUID     NOT NULL,
    object_key TEXT     NOT NULL,
    PRIMARY KEY (review_id, position),
    CONSTRAINT review_images_asset_key UNIQUE (review_id, asset_id),
    CONSTRAINT review_images_position_check CHECK (position BETWEEN 0 AND 3),
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

-- 商品評分的待重算佇列（transactional outbox，D3）：評價集合改變的 transaction（提交、隱藏、還原、刪帳清除）在同 transaction 登記商品，
-- 週期 worker 以 SKIP LOCKED 認領到期列、寫租約，鎖 catalog 商品列後無鎖讀統計並寫回絕對值。
-- 必要條件：catalog 排在 review 之前（platform/lockorder），review 鎖了自己的列之後不能再等 catalog 的列，所以評分不能在評價的 transaction 內寫。
-- seq 是請求序號：每次登記加一；worker 完成時只在 seq 仍是認領時讀到的值才刪列，變了就重新排（處理中到的請求不遺失）。
-- claimed_until 是認領的租約，claim_token 是認領的擁有者憑證（每次認領換一個）：完成、失敗與重排都要同時對上 seq 與 claim_token，租約過期被別人重新認領後，舊 worker 的收尾不動這一列。
-- priority：0 是評價寫入的即時請求，1 是週期對帳登記的；認領先 0 後 1，所以對帳的大量待辦不會擋住即時請求。
-- attempts、last_error 記失敗；failed_at 非空表示超過重試上限而停手（列保留供查，只有即時請求會重置，對帳不動它）。
CREATE TABLE review.rating_recompute_queue (
    product_id    UUID        PRIMARY KEY,
    seq           BIGINT      NOT NULL DEFAULT 1,
    due_at        TIMESTAMPTZ NOT NULL,
    claimed_until TIMESTAMPTZ,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT,
    failed_at     TIMESTAMPTZ,
    priority      SMALLINT    NOT NULL DEFAULT 0,
    claim_token   UUID,
    CONSTRAINT rating_recompute_queue_priority_check CHECK (priority IN (0, 1)),
    CONSTRAINT rating_recompute_queue_seq_check CHECK (seq >= 1),
    CONSTRAINT rating_recompute_queue_attempts_check CHECK (attempts >= 0)
);
CREATE INDEX rating_recompute_queue_due ON review.rating_recompute_queue (priority, due_at) WHERE failed_at IS NULL;
