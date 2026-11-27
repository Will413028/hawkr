
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA adminaudit;

CREATE TABLE adminaudit.records (
    id          BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL,
    admin_id    BIGINT      NOT NULL,
    action      TEXT        NOT NULL,
    target_type TEXT        NOT NULL,
    target_id   TEXT        NOT NULL,
    detail      JSONB       NOT NULL DEFAULT '{}',
    CONSTRAINT records_admin_id_check CHECK (admin_id > 0),
    CONSTRAINT records_action_check CHECK (char_length(action) BETWEEN 1 AND 200),
    CONSTRAINT records_target_type_check CHECK (target_type ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT records_target_id_check CHECK (char_length(target_id) BETWEEN 1 AND 255),
    CONSTRAINT records_detail_check CHECK (jsonb_typeof(detail) = 'object' AND octet_length(detail::text) <= 2048)
);
CREATE INDEX records_by_admin ON adminaudit.records (admin_id, occurred_at);
CREATE INDEX records_by_action ON adminaudit.records (action, occurred_at);
CREATE INDEX records_by_target ON adminaudit.records (target_type, target_id, occurred_at);
CREATE INDEX records_by_time ON adminaudit.records (occurred_at);
