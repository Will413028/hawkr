
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA inbox;

CREATE TABLE inbox.events (
    provider      TEXT        NOT NULL,
    event_id      TEXT        NOT NULL,
    route_key     TEXT        NOT NULL,
    payload       BYTEA       NOT NULL,
    received_at   TIMESTAMPTZ NOT NULL,
    purge_after   TIMESTAMPTZ NOT NULL,
    dispatched_at TIMESTAMPTZ,
    processed_at  TIMESTAMPTZ,
    processed_by  TEXT,
    dead_at       TIMESTAMPTZ,
    dead_reason   TEXT,
    PRIMARY KEY (provider, event_id),
    CONSTRAINT events_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT events_event_id_check CHECK (char_length(event_id) BETWEEN 1 AND 255),
    CONSTRAINT events_route_key_check CHECK (char_length(route_key) BETWEEN 1 AND 255),
    CONSTRAINT events_purge_after_check CHECK (purge_after > received_at),
    CONSTRAINT events_processed_check CHECK ((processed_at IS NULL) = (processed_by IS NULL)),
    CONSTRAINT events_dead_check CHECK ((dead_at IS NULL) = (dead_reason IS NULL) AND (dead_at IS NULL OR processed_at IS NULL)),
    CONSTRAINT events_dead_reason_check CHECK (dead_reason IS NULL OR dead_reason IN ('undispatched', 'unprocessed')),
    CONSTRAINT events_processed_by_check CHECK (processed_by IS NULL OR char_length(processed_by) BETWEEN 1 AND 100)
);
CREATE INDEX events_pending ON inbox.events (provider, route_key, received_at) WHERE processed_at IS NULL AND dead_at IS NULL;
CREATE INDEX events_undispatched ON inbox.events (received_at) WHERE processed_at IS NULL AND dead_at IS NULL AND dispatched_at IS NULL;
CREATE INDEX events_purge ON inbox.events (purge_after) WHERE processed_at IS NOT NULL OR dead_at IS NOT NULL;
CREATE INDEX events_dead ON inbox.events (dead_at) WHERE dead_at IS NOT NULL;
