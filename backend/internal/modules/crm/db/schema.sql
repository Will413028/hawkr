
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA crm;

CREATE TABLE crm.customers (
    store_id       BIGINT      NOT NULL,
    buyer_id       BIGINT      NOT NULL,
    customer_type  TEXT        NOT NULL DEFAULT 'NORMAL',
    total_spent    BIGINT      NOT NULL DEFAULT 0,
    order_count    INTEGER     NOT NULL DEFAULT 0,
    first_order_at TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id),
    CONSTRAINT customers_store_check CHECK (store_id > 0),
    CONSTRAINT customers_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT customers_type_check CHECK (customer_type IN ('NORMAL', 'VIP')),
    CONSTRAINT customers_total_spent_check CHECK (total_spent >= 0),
    CONSTRAINT customers_order_count_check CHECK (order_count >= 0)
);
CREATE INDEX customers_by_spent ON crm.customers (store_id, total_spent DESC, buyer_id DESC);
CREATE INDEX customers_by_orders ON crm.customers (store_id, order_count DESC, buyer_id DESC);
CREATE INDEX customers_by_first_order ON crm.customers (store_id, first_order_at DESC, buyer_id DESC);
CREATE INDEX customers_by_buyer ON crm.customers (buyer_id);

CREATE TABLE crm.tags (
    id         UUID        PRIMARY KEY,
    store_id   BIGINT      NOT NULL,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT tags_store_name_key UNIQUE (store_id, name),
    CONSTRAINT tags_store_id_key UNIQUE (store_id, id),
    CONSTRAINT tags_store_check CHECK (store_id > 0),
    CONSTRAINT tags_name_check CHECK (char_length(name) >= 1 AND char_length(name) <= 50 AND name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);

CREATE TABLE crm.customer_tags (
    store_id   BIGINT      NOT NULL,
    buyer_id   BIGINT      NOT NULL,
    tag_id     UUID        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id, tag_id),
    CONSTRAINT customer_tags_tag_fkey FOREIGN KEY (store_id, tag_id) REFERENCES crm.tags (store_id, id) ON DELETE CASCADE,
    CONSTRAINT customer_tags_customer_fkey FOREIGN KEY (store_id, buyer_id) REFERENCES crm.customers (store_id, buyer_id) ON DELETE CASCADE
);
CREATE INDEX customer_tags_by_tag ON crm.customer_tags (store_id, tag_id, buyer_id);

CREATE TABLE crm.order_nets (
    order_no      TEXT        PRIMARY KEY,
    store_id      BIGINT      NOT NULL,
    buyer_id      BIGINT      NOT NULL,
    completed     BOOLEAN     NOT NULL DEFAULT false,
    full_refunded BOOLEAN     NOT NULL DEFAULT false,
    net_amount    BIGINT      NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT order_nets_order_no_check CHECK (char_length(order_no) >= 1 AND char_length(order_no) <= 128),
    CONSTRAINT order_nets_store_check CHECK (store_id > 0),
    CONSTRAINT order_nets_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX order_nets_by_customer ON crm.order_nets (store_id, buyer_id);
CREATE INDEX order_nets_by_buyer ON crm.order_nets (buyer_id);

CREATE TABLE crm.ledger (
    id        UUID PRIMARY KEY DEFAULT uuidv7(),
    dedup_key TEXT NOT NULL,
    order_no  TEXT NOT NULL REFERENCES crm.order_nets (order_no) ON DELETE CASCADE,
    CONSTRAINT ledger_dedup_key_key UNIQUE (dedup_key),
    CONSTRAINT ledger_dedup_key_check CHECK (char_length(dedup_key) >= 1 AND char_length(dedup_key) <= 128)
);
