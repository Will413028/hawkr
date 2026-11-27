
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA review;

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
