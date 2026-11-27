
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA notification;

CREATE TABLE notification.intents (
    id              UUID        PRIMARY KEY,
    type            TEXT        NOT NULL,
    idempotency_key TEXT        NOT NULL,
    params          JSONB       NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT intents_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT intents_type_check CHECK (type ~ '^[a-z][a-z0-9_]{0,62}$'),
    CONSTRAINT intents_idempotency_key_check CHECK (char_length(idempotency_key) BETWEEN 1 AND 200)
);
CREATE INDEX intents_created ON notification.intents (created_at);

CREATE TABLE notification.announcements (
    id            UUID        PRIMARY KEY,
    audiences     TEXT[]      NOT NULL,
    title         TEXT        NOT NULL,
    body          TEXT        NOT NULL,
    admin_id      BIGINT      NOT NULL,
    request_id    UUID,
    inbox_count   INTEGER     NOT NULL DEFAULT 0,
    inbox_done_at TIMESTAMPTZ,
    push_done_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT announcements_request_key UNIQUE (admin_id, request_id),
    CONSTRAINT announcements_audiences_check CHECK (audiences IN (ARRAY['BUYER'], ARRAY['STORE'], ARRAY['BUYER', 'STORE'])),
    CONSTRAINT announcements_title_check CHECK (char_length(title) BETWEEN 1 AND 100 AND title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT announcements_body_check CHECK (char_length(body) BETWEEN 1 AND 2000 AND body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT announcements_admin_check CHECK (admin_id > 0),
    CONSTRAINT announcements_inbox_count_check CHECK (inbox_count >= 0)
);
CREATE INDEX announcements_listing ON notification.announcements (created_at DESC, id DESC);

CREATE TABLE notification.inbox_items (
    id                UUID        PRIMARY KEY,
    account_id        BIGINT      NOT NULL,
    audience          TEXT        NOT NULL,
    store_id          BIGINT,
    type              TEXT        NOT NULL,
    params            JSONB       NOT NULL,
    aggregation_key   TEXT,
    actor_account_ids BIGINT[]    NOT NULL DEFAULT '{}',
    occurred_at       TIMESTAMPTZ NOT NULL,
    read_at           TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL,
    updated_at        TIMESTAMPTZ NOT NULL,
    announcement_id   UUID REFERENCES notification.announcements (id),
    CONSTRAINT inbox_items_account_check CHECK (account_id > 0),
    CONSTRAINT inbox_items_audience_check CHECK (audience IN ('BUYER', 'STORE')),
    CONSTRAINT inbox_items_store_check CHECK ((audience = 'STORE') = (store_id IS NOT NULL) AND (store_id IS NULL OR store_id > 0)),
    CONSTRAINT inbox_items_type_check CHECK (type ~ '^[a-z][a-z0-9_]{0,62}$'),
    CONSTRAINT inbox_items_aggregation_key_check CHECK (aggregation_key IS NULL OR char_length(aggregation_key) BETWEEN 1 AND 200),
    CONSTRAINT inbox_items_updated_check CHECK (updated_at >= created_at),
    CONSTRAINT inbox_items_announcement_check CHECK ((type = 'announcement') = (announcement_id IS NOT NULL))
);
CREATE INDEX inbox_items_recipient ON notification.inbox_items (account_id, audience, store_id, updated_at DESC, id DESC);
CREATE INDEX inbox_items_unread ON notification.inbox_items (account_id, audience, store_id) WHERE read_at IS NULL;
CREATE UNIQUE INDEX inbox_items_unread_aggregation ON notification.inbox_items (account_id, audience, store_id, aggregation_key) NULLS NOT DISTINCT WHERE aggregation_key IS NOT NULL AND read_at IS NULL;
CREATE INDEX inbox_items_updated ON notification.inbox_items (updated_at);
CREATE UNIQUE INDEX inbox_items_announcement_recipient ON notification.inbox_items (announcement_id, account_id, audience, store_id) NULLS NOT DISTINCT WHERE announcement_id IS NOT NULL;

CREATE TABLE notification.inbox_item_intents (
    intent_id     UUID PRIMARY KEY REFERENCES notification.intents (id) ON DELETE CASCADE,
    inbox_item_id UUID NOT NULL REFERENCES notification.inbox_items (id) ON DELETE CASCADE
);
CREATE INDEX inbox_item_intents_item ON notification.inbox_item_intents (inbox_item_id);

CREATE TABLE notification.deliveries (
    id             UUID        PRIMARY KEY,
    intent_id      UUID        NOT NULL REFERENCES notification.intents (id) ON DELETE RESTRICT,
    account_id     BIGINT      NOT NULL,
    audience       TEXT        NOT NULL,
    store_id       BIGINT,
    channel        TEXT        NOT NULL,
    status         TEXT        NOT NULL,
    locale         TEXT,
    sent_title     TEXT,
    sent_body      TEXT,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL,
    sent_at        TIMESTAMPTZ,
    CONSTRAINT deliveries_recipient_channel_key UNIQUE NULLS NOT DISTINCT (intent_id, account_id, audience, store_id, channel),
    CONSTRAINT deliveries_account_check CHECK (account_id > 0),
    CONSTRAINT deliveries_audience_check CHECK (audience IN ('BUYER', 'STORE')),
    CONSTRAINT deliveries_store_check CHECK ((audience = 'STORE') = (store_id IS NOT NULL) AND (store_id IS NULL OR store_id > 0)),
    CONSTRAINT deliveries_channel_check CHECK (channel IN ('PUSH', 'EMAIL')),
    CONSTRAINT deliveries_status_check CHECK (status IN ('PENDING', 'SENT', 'FAILED_PERMANENT')),
    CONSTRAINT deliveries_locale_check CHECK (locale IS NULL OR locale ~ '^[a-z]{2,3}(-[A-Z]{2})?$'),
    CONSTRAINT deliveries_sent_check CHECK ((status = 'SENT') = (sent_at IS NOT NULL)),
    CONSTRAINT deliveries_sent_text_check CHECK (status = 'SENT' OR (locale IS NULL AND sent_title IS NULL AND sent_body IS NULL)),
    CONSTRAINT deliveries_failure_check CHECK ((status = 'FAILED_PERMANENT') = (failure_reason IS NOT NULL))
);
CREATE INDEX deliveries_recipient ON notification.deliveries (account_id, audience, store_id);
CREATE INDEX deliveries_status ON notification.deliveries (status, created_at) WHERE status <> 'SENT';
CREATE INDEX deliveries_created ON notification.deliveries (created_at);

CREATE TABLE notification.device_tokens (
    token              TEXT        PRIMARY KEY,
    account_id         BIGINT      NOT NULL,
    app                TEXT        NOT NULL,
    last_registered_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT device_tokens_token_check CHECK (char_length(token) BETWEEN 1 AND 4096),
    CONSTRAINT device_tokens_account_check CHECK (account_id > 0),
    CONSTRAINT device_tokens_app_check CHECK (app IN ('BUYER', 'SELLER'))
);
CREATE INDEX device_tokens_account ON notification.device_tokens (account_id, app);
CREATE INDEX device_tokens_last_registered ON notification.device_tokens (last_registered_at);

CREATE TABLE notification.preference_overrides (
    account_id BIGINT      NOT NULL,
    audience   TEXT        NOT NULL,
    category   TEXT        NOT NULL,
    channel    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (account_id, audience, category, channel),
    CONSTRAINT preference_overrides_account_check CHECK (account_id > 0),
    CONSTRAINT preference_overrides_audience_check CHECK (audience IN ('BUYER', 'STORE')),
    CONSTRAINT preference_overrides_category_check CHECK (category ~ '^[A-Z][A-Z0-9_]{0,62}$'),
    CONSTRAINT preference_overrides_channel_check CHECK (channel IN ('INBOX', 'PUSH', 'EMAIL'))
);

CREATE TABLE notification.email_suppressions (
    address_hash BYTEA       PRIMARY KEY,
    reason       TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT email_suppressions_address_hash_check CHECK (octet_length(address_hash) = 32),
    CONSTRAINT email_suppressions_reason_check CHECK (reason IN ('HARD_BOUNCE', 'COMPLAINT'))
);
