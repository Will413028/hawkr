
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA catalog;

CREATE TABLE catalog.categories (
    id                  UUID    PRIMARY KEY,
    parent_id           UUID    REFERENCES catalog.categories (id),
    name                JSONB   NOT NULL,
    shopify_taxonomy_id TEXT,
    position            INTEGER NOT NULL,
    CONSTRAINT categories_name_check CHECK (
        jsonb_typeof(name) = 'object'
        AND COALESCE(jsonb_typeof(name -> 'zh-TW'), '') = 'string' AND char_length(name ->> 'zh-TW') >= 1
        AND (NOT name ? 'en' OR (COALESCE(jsonb_typeof(name -> 'en'), '') = 'string' AND char_length(name ->> 'en') >= 1))),
    CONSTRAINT categories_position_check CHECK (position >= 0),
    CONSTRAINT categories_taxonomy_shape_check CHECK (parent_id IS NOT NULL OR shopify_taxonomy_id IS NULL),
    CONSTRAINT categories_not_self_parent_check CHECK (parent_id IS DISTINCT FROM id)
);

CREATE UNIQUE INDEX categories_shopify_taxonomy_key ON catalog.categories (shopify_taxonomy_id) WHERE shopify_taxonomy_id IS NOT NULL;
CREATE UNIQUE INDEX categories_sibling_name_key ON catalog.categories (COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), (name ->> 'zh-TW'));
CREATE INDEX categories_parent_idx ON catalog.categories (parent_id, position);

CREATE TABLE catalog.products (
    id                 UUID             PRIMARY KEY,
    store_id           BIGINT           NOT NULL,
    category_id        UUID             NOT NULL REFERENCES catalog.categories (id),
    title              TEXT             NOT NULL,
    description        JSONB            NOT NULL DEFAULT '[]',
    status             TEXT             NOT NULL,
    first_published_at TIMESTAMPTZ,
    deleted_at         TIMESTAMPTZ,
    min_price          BIGINT           NOT NULL,
    max_price          BIGINT           NOT NULL,
    sales_count        INTEGER          NOT NULL DEFAULT 0,
    favorite_count     INTEGER          NOT NULL DEFAULT 0,
    rating_average     DOUBLE PRECISION NOT NULL DEFAULT 0,
    review_count       INTEGER          NOT NULL DEFAULT 0,
    searchable_text    TEXT             NOT NULL DEFAULT '',
    content_version    BIGINT           NOT NULL DEFAULT 1,
    low_stock_threshold INTEGER,
    request_id         UUID,
    request_hash       BYTEA,
    created_at         TIMESTAMPTZ      NOT NULL,
    updated_at         TIMESTAMPTZ      NOT NULL,
    CONSTRAINT products_store_check CHECK (store_id > 0),
    CONSTRAINT products_version_check CHECK (content_version >= 1),
    CONSTRAINT products_low_stock_check CHECK (low_stock_threshold IS NULL OR (low_stock_threshold >= 0 AND low_stock_threshold <= 9999)),
    CONSTRAINT products_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT products_title_check CHECK (char_length(title) >= 1 AND char_length(title) <= 100 AND title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT products_description_check CHECK (jsonb_typeof(description) = 'array'),
    CONSTRAINT products_status_check CHECK (status IN ('DRAFT', 'ACTIVE', 'ARCHIVED')),
    CONSTRAINT products_first_published_check CHECK (status <> 'ACTIVE' OR first_published_at IS NOT NULL),
    CONSTRAINT products_price_range_check CHECK (min_price >= 0 AND max_price >= min_price),
    CONSTRAINT products_counts_check CHECK (sales_count >= 0 AND favorite_count >= 0 AND review_count >= 0),
    CONSTRAINT products_rating_check CHECK (rating_average >= 0 AND rating_average <= 5 AND (review_count > 0 OR rating_average = 0))
);

CREATE INDEX products_store_idx ON catalog.products (store_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX products_category_idx ON catalog.products (category_id, id) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_new_idx ON catalog.products (first_published_at DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE UNIQUE INDEX products_request_key ON catalog.products (store_id, request_id) WHERE request_id IS NOT NULL;
CREATE INDEX products_search_idx ON catalog.products USING gin (searchable_text public.gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX products_public_sales_idx ON catalog.products (sales_count DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_price_asc_idx ON catalog.products ((-min_price) DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_price_desc_idx ON catalog.products (max_price DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';
CREATE INDEX products_public_rating_idx ON catalog.products ((round(rating_average * 1000)::bigint) DESC, id DESC) WHERE deleted_at IS NULL AND status = 'ACTIVE';

CREATE TABLE catalog.product_option_axes (
    product_id UUID    NOT NULL REFERENCES catalog.products (id),
    position   INTEGER NOT NULL,
    name       TEXT    NOT NULL,
    PRIMARY KEY (product_id, position),
    CONSTRAINT option_axes_position_check CHECK (position IN (1, 2)),
    CONSTRAINT option_axes_name_check CHECK (char_length(name) >= 1 AND char_length(name) <= 20 AND name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    UNIQUE (product_id, name)
);

CREATE TABLE catalog.product_option_values (
    product_id    UUID    NOT NULL,
    axis_position INTEGER NOT NULL,
    position      INTEGER NOT NULL,
    value         TEXT    NOT NULL,
    PRIMARY KEY (product_id, axis_position, value),
    FOREIGN KEY (product_id, axis_position) REFERENCES catalog.product_option_axes (product_id, position),
    CONSTRAINT option_values_position_check CHECK (position >= 0),
    CONSTRAINT option_values_value_check CHECK (char_length(value) >= 1 AND char_length(value) <= 40 AND value !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    UNIQUE (product_id, axis_position, position) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE catalog.product_skus (
    id               UUID        PRIMARY KEY,
    product_id       UUID        NOT NULL REFERENCES catalog.products (id),
    option1_value    TEXT,
    option2_value    TEXT,
    price            BIGINT      NOT NULL,
    compare_at_price BIGINT,
    stock_qty        INTEGER     NOT NULL DEFAULT 0,
    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    CONSTRAINT skus_price_check CHECK (price > 0),
    CONSTRAINT skus_compare_at_check CHECK (compare_at_price IS NULL OR compare_at_price > price),
    CONSTRAINT skus_stock_check CHECK (stock_qty >= 0),
    CONSTRAINT skus_option_order_check CHECK (option2_value IS NULL OR option1_value IS NOT NULL)
);

CREATE UNIQUE INDEX skus_combination_key ON catalog.product_skus (product_id, COALESCE(option1_value, ''), COALESCE(option2_value, '')) WHERE deleted_at IS NULL;
CREATE INDEX skus_product_idx ON catalog.product_skus (product_id) WHERE deleted_at IS NULL;

CREATE TABLE catalog.stock_operations (
    id                  UUID        PRIMARY KEY,
    idempotency_key     TEXT        NOT NULL,
    kind                TEXT        NOT NULL,
    store_id            BIGINT,
    deduct_operation_id UUID        REFERENCES catalog.stock_operations (id),
    request_hash        BYTEA,
    created_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT stock_operations_key_check CHECK (char_length(idempotency_key) >= 1 AND char_length(idempotency_key) <= 200),
    CONSTRAINT stock_operations_kind_check CHECK (kind IN ('DEDUCT', 'RESTORE', 'ADJUST', 'SET')),
    CONSTRAINT stock_operations_restore_ref_check CHECK ((kind = 'RESTORE') = (deduct_operation_id IS NOT NULL)),
    CONSTRAINT stock_operations_store_check CHECK ((kind IN ('ADJUST', 'SET')) = (store_id IS NOT NULL)),
    CONSTRAINT stock_operations_hash_check CHECK ((kind IN ('ADJUST', 'SET')) = (request_hash IS NOT NULL))
);

CREATE UNIQUE INDEX stock_operations_global_key ON catalog.stock_operations (idempotency_key) WHERE store_id IS NULL;
CREATE UNIQUE INDEX stock_operations_store_key ON catalog.stock_operations (store_id, idempotency_key) WHERE store_id IS NOT NULL;
CREATE INDEX stock_operations_deduct_idx ON catalog.stock_operations (deduct_operation_id) WHERE deduct_operation_id IS NOT NULL;

CREATE TABLE catalog.stock_movements (
    operation_id UUID    NOT NULL REFERENCES catalog.stock_operations (id),
    sku_id       UUID    NOT NULL REFERENCES catalog.product_skus (id),
    delta        INTEGER NOT NULL,
    PRIMARY KEY (operation_id, sku_id),
    CONSTRAINT stock_movements_delta_check CHECK (delta <> 0)
);

CREATE INDEX stock_movements_sku_idx ON catalog.stock_movements (sku_id);

CREATE TABLE catalog.product_images (
    product_id UUID    NOT NULL REFERENCES catalog.products (id),
    kind       TEXT    NOT NULL,
    position   INTEGER NOT NULL,
    asset_id   UUID    NOT NULL,
    object_key TEXT    NOT NULL,
    alt        TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (product_id, kind, position),
    CONSTRAINT product_images_kind_check CHECK (kind IN ('GALLERY', 'DESCRIPTION')),
    CONSTRAINT product_images_position_check CHECK (position >= 0),
    CONSTRAINT product_images_alt_check CHECK (char_length(alt) <= 200),
    UNIQUE (product_id, asset_id)
);

CREATE TABLE catalog.product_hashtags (
    product_id   UUID    NOT NULL REFERENCES catalog.products (id),
    name         TEXT    NOT NULL,
    display_name TEXT    NOT NULL,
    position     INTEGER NOT NULL,
    PRIMARY KEY (product_id, name),
    CONSTRAINT product_hashtags_name_check CHECK (char_length(name) >= 1 AND char_length(name) <= 40),
    CONSTRAINT product_hashtags_position_check CHECK (position >= 0)
);


CREATE TABLE catalog.sales_orders (
    order_no    TEXT        PRIMARY KEY,
    recorded_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT sales_orders_no_check CHECK (char_length(order_no) >= 1 AND char_length(order_no) <= 200)
);

CREATE TABLE catalog.restock_queue (
    sku_id        UUID        PRIMARY KEY,
    due_at        TIMESTAMPTZ NOT NULL,
    requested_at  TIMESTAMPTZ NOT NULL,
    claimed_until TIMESTAMPTZ,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT,
    failed_at     TIMESTAMPTZ,
    CONSTRAINT restock_queue_attempts_check CHECK (attempts >= 0)
);

CREATE INDEX restock_queue_due_idx ON catalog.restock_queue (due_at) WHERE failed_at IS NULL;

CREATE TABLE catalog.restock_purge_queue (
    sku_id        UUID        PRIMARY KEY,
    due_at        TIMESTAMPTZ NOT NULL,
    requested_at  TIMESTAMPTZ NOT NULL,
    claimed_until TIMESTAMPTZ,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT,
    failed_at     TIMESTAMPTZ,
    CONSTRAINT restock_purge_queue_attempts_check CHECK (attempts >= 0)
);

CREATE INDEX restock_purge_queue_due_idx ON catalog.restock_purge_queue (due_at) WHERE failed_at IS NULL;
