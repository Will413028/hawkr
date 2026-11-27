
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA assistant;

CREATE TABLE assistant.sessions (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    id TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_name, user_id, id),
    CONSTRAINT sessions_key_check CHECK (app_name <> '' AND user_id <> '' AND id <> ''),
    CONSTRAINT sessions_state_check CHECK (jsonb_typeof(state) = 'object')
);

CREATE TABLE assistant.events (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    id TEXT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    seq BIGINT NOT NULL GENERATED ALWAYS AS IDENTITY,
    event JSONB NOT NULL,
    PRIMARY KEY (app_name, user_id, session_id, id),
    FOREIGN KEY (app_name, user_id, session_id) REFERENCES assistant.sessions (app_name, user_id, id) ON DELETE CASCADE,
    CONSTRAINT events_id_check CHECK (id <> ''),
    CONSTRAINT events_event_check CHECK (jsonb_typeof(event) = 'object')
);
CREATE INDEX events_session_time ON assistant.events (app_name, user_id, session_id, timestamp, seq);

CREATE TABLE assistant.app_states (
    app_name TEXT PRIMARY KEY,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT app_states_state_check CHECK (jsonb_typeof(state) = 'object')
);

CREATE TABLE assistant.user_states (
    app_name TEXT NOT NULL,
    user_id TEXT NOT NULL,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (app_name, user_id),
    CONSTRAINT user_states_state_check CHECK (jsonb_typeof(state) = 'object')
);
