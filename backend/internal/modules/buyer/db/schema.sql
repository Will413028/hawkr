CREATE SCHEMA IF NOT EXISTS public;
CREATE SCHEMA buyer;

CREATE TABLE buyer.buyers (
    buyer_id       BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     BIGINT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'ACTIVE',
    version        BIGINT NOT NULL DEFAULT 1,
    display_name   TEXT,
    phone          TEXT,
    gender         TEXT,
    avatar         TEXT,
    avatar_asset_id UUID,
    birth_date     DATE,
    city           TEXT,
    job            TEXT,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    deleted_at     TIMESTAMPTZ,
    pii_cleared_at  TIMESTAMPTZ,
    CONSTRAINT buyers_status_check CHECK (status IN ('ACTIVE', 'PENDING_DELETION', 'CLOSED')),
    CONSTRAINT buyers_gender_check CHECK (gender IN ('MALE', 'FEMALE', 'OTHER')),
    CONSTRAINT buyers_deleted_check CHECK ((status = 'CLOSED') = (deleted_at IS NOT NULL)),
    CONSTRAINT buyers_pii_cleared_check CHECK (status = 'CLOSED' OR pii_cleared_at IS NULL),
    CONSTRAINT buyers_avatar_asset_check CHECK ((avatar IS NULL) = (avatar_asset_id IS NULL)),
    CONSTRAINT buyers_display_name_check CHECK (
        (pii_cleared_at IS NOT NULL AND display_name IS NULL) OR
        (pii_cleared_at IS NULL AND display_name IS NOT NULL AND char_length(display_name) BETWEEN 1 AND 100)
    )
);
CREATE UNIQUE INDEX buyers_account ON buyer.buyers (account_id);

CREATE TABLE buyer.shipping_addresses (
    address_id     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    buyer_id       BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    receiver_name  TEXT NOT NULL,
    receiver_phone TEXT NOT NULL,
    town_code      TEXT NOT NULL,
    address_detail TEXT NOT NULL,
    is_default     BOOLEAN NOT NULL,
    version        BIGINT NOT NULL DEFAULT 1,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT shipping_addresses_town_code_check CHECK (town_code ~ '^[0-9]{8}$'),
    CONSTRAINT shipping_addresses_address_detail_check CHECK (char_length(address_detail) BETWEEN 1 AND 255)
);
CREATE UNIQUE INDEX addresses_default ON buyer.shipping_addresses (buyer_id) WHERE is_default;

CREATE TABLE buyer.favorites (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    product_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, product_id)
);

CREATE TABLE buyer.visit_history (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    product_id UUID NOT NULL,
    visited_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, product_id)
);

CREATE TABLE buyer.seller_subscriptions (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    store_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, store_id)
);
CREATE INDEX subscriptions_store ON buyer.seller_subscriptions (store_id, buyer_id);

CREATE TABLE buyer.blocked_buyers (
    blocker_buyer_id BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    blocked_buyer_id BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    created_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (blocker_buyer_id, blocked_buyer_id),
    CONSTRAINT blocked_buyers_not_self_check CHECK (blocker_buyer_id <> blocked_buyer_id)
);
CREATE INDEX blocked_buyers_blocked ON buyer.blocked_buyers (blocked_buyer_id, blocker_buyer_id);

CREATE TABLE buyer.blocked_stores (
    blocker_buyer_id BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    store_id         BIGINT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (blocker_buyer_id, store_id),
    CONSTRAINT blocked_stores_store_id_check CHECK (store_id > 0)
);
CREATE INDEX blocked_stores_store ON buyer.blocked_stores (store_id, blocker_buyer_id);

CREATE TABLE buyer.restock_subscriptions (
    buyer_id   BIGINT NOT NULL REFERENCES buyer.buyers (buyer_id),
    sku_id     UUID NOT NULL,
    product_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (buyer_id, sku_id)
);
CREATE INDEX restock_sku ON buyer.restock_subscriptions (sku_id, buyer_id);

