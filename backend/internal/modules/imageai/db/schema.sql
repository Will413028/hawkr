
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA imageai;

CREATE TABLE imageai.styles (
    id                 UUID        PRIMARY KEY,
    name               TEXT        NOT NULL,
    label              TEXT        NOT NULL,
    prompt             TEXT        NOT NULL,
    category           TEXT        NOT NULL,
    thumbnail_key      TEXT        NOT NULL DEFAULT '',
    thumbnail_asset_id UUID,
    display_order      INT         NOT NULL DEFAULT 0,
    is_default         BOOLEAN     NOT NULL DEFAULT FALSE,
    enabled            BOOLEAN     NOT NULL DEFAULT TRUE,
    version            BIGINT      NOT NULL DEFAULT 1,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT styles_category_check CHECK (category IN ('PRODUCT', 'POST')),
    CONSTRAINT styles_name_check CHECK (char_length(name) >= 1 AND name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT styles_label_check CHECK (char_length(label) >= 1 AND label !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT styles_prompt_check CHECK (char_length(prompt) >= 1 AND char_length(prompt) <= 500 AND prompt !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT styles_default_enabled_check CHECK (NOT is_default OR enabled),
    CONSTRAINT styles_category_name_key UNIQUE (category, name)
);
CREATE UNIQUE INDEX styles_default_per_category ON imageai.styles (category) WHERE is_default;

COMMENT ON COLUMN imageai.styles.version IS '樂觀鎖的版本，對外是 etag；每次修改加一（ADR 2026-09-25-optimistic-concurrency-etag）';
COMMENT ON COLUMN imageai.styles.prompt IS '提示詞，最多 500 個字元（A9-D40，以 Unicode 字元計）';

CREATE TABLE imageai.quotas (
    store_id     BIGINT      PRIMARY KEY,
    weekly_limit INT         NOT NULL,
    memo         TEXT        NOT NULL DEFAULT '',
    version      BIGINT      NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT quotas_store_check CHECK (store_id > 0),
    CONSTRAINT quotas_weekly_limit_check CHECK (weekly_limit >= 0),
    CONSTRAINT quotas_memo_check CHECK (memo !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);

COMMENT ON COLUMN imageai.quotas.version IS '樂觀鎖的版本，對外是 etag；每次修改加一';

CREATE TABLE imageai.usage (
    store_id     BIGINT      PRIMARY KEY,
    period_start TIMESTAMPTZ NOT NULL,
    used         INT         NOT NULL DEFAULT 0,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT usage_store_check CHECK (store_id > 0),
    CONSTRAINT usage_used_check CHECK (used >= 0)
);
