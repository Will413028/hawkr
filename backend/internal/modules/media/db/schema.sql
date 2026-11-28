
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA media;

CREATE TABLE media.assets (
    id           UUID        PRIMARY KEY,
    visibility   TEXT        NOT NULL,
    object_key   TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL,
    state        TEXT        NOT NULL,
    kek_id       TEXT,
    wrapped_dek  BYTEA,
    chunk_count  INTEGER,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT assets_object_key_key UNIQUE (object_key),
    CONSTRAINT assets_visibility_check CHECK (visibility IN ('PUBLIC', 'SECURE')),
    CONSTRAINT assets_state_check CHECK (state IN ('STAGED', 'DELETING', 'DELETED')),
    CONSTRAINT assets_content_type_check CHECK (content_type <> ''),
    CONSTRAINT assets_size_check CHECK (size_bytes >= 0),
    CONSTRAINT assets_kek_id_check CHECK ((visibility = 'SECURE') = (kek_id IS NOT NULL)),
    CONSTRAINT assets_wrapped_dek_check CHECK ((visibility = 'SECURE') = (wrapped_dek IS NOT NULL)),
    CONSTRAINT assets_chunk_count_check CHECK ((visibility = 'SECURE') = (chunk_count IS NOT NULL) AND (chunk_count IS NULL OR chunk_count > 0)),
    CONSTRAINT assets_object_key_prefix_check CHECK (
        (visibility = 'PUBLIC' AND object_key LIKE 'public/%') OR (visibility = 'SECURE' AND object_key LIKE 'secure/%')
    )
);
CREATE INDEX assets_deleting ON media.assets (id) WHERE state = 'DELETING';

CREATE TABLE media.staged_uploads (
    id                    UUID        PRIMARY KEY,
    token_hash            BYTEA       NOT NULL,
    uploader_id           BIGINT      NOT NULL,
    uploader_admin        BOOLEAN     NOT NULL,
    delegated_store_id    BIGINT,
    visibility            TEXT        NOT NULL,
    declared_size         BIGINT      NOT NULL,
    declared_content_type TEXT,
    declared_md5          TEXT,
    state                 TEXT        NOT NULL,
    reject_reason         TEXT,
    tmp_key               TEXT,
    final_key             TEXT,
    asset_id              UUID,
    planned_asset_id      UUID,
    chunk_count           INTEGER,
    next_index            INTEGER,
    kek_id                TEXT,
    wrapped_dek           BYTEA,
    expires_at            TIMESTAMPTZ NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL,
    updated_at            TIMESTAMPTZ NOT NULL,
    CONSTRAINT staged_uploads_token_hash_key UNIQUE (token_hash),
    CONSTRAINT staged_uploads_tmp_key_key UNIQUE (tmp_key),
    CONSTRAINT staged_uploads_final_key_key UNIQUE (final_key),
    CONSTRAINT staged_uploads_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT staged_uploads_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT staged_uploads_uploader_check CHECK (uploader_id > 0),
    CONSTRAINT staged_uploads_delegated_store_check CHECK (delegated_store_id IS NULL OR (delegated_store_id > 0 AND uploader_admin)),
    CONSTRAINT staged_uploads_visibility_check CHECK (visibility IN ('PUBLIC', 'SECURE')),
    CONSTRAINT staged_uploads_declared_size_check CHECK (declared_size > 0),
    CONSTRAINT staged_uploads_declared_content_type_check CHECK (
        (declared_content_type IS NULL OR declared_content_type <> '')
        AND (visibility = 'SECURE' OR declared_content_type IS NOT NULL)
        AND (state NOT IN ('READY', 'CONFIRMED') OR declared_content_type IS NOT NULL)
    ),
    CONSTRAINT staged_uploads_planned_asset_id_key UNIQUE (planned_asset_id),
    CONSTRAINT staged_uploads_chunks_check CHECK (
        (visibility = 'SECURE') = (planned_asset_id IS NOT NULL)
        AND (visibility = 'SECURE') = (chunk_count IS NOT NULL)
        AND (visibility = 'SECURE') = (next_index IS NOT NULL)
        AND (visibility = 'SECURE') = (kek_id IS NOT NULL)
        AND (visibility = 'SECURE') = (wrapped_dek IS NOT NULL)
        AND (chunk_count IS NULL OR chunk_count > 0)
        AND (next_index IS NULL OR (next_index >= 0 AND next_index <= chunk_count))
    ),
    CONSTRAINT staged_uploads_ready_chunks_check CHECK (
        visibility <> 'SECURE' OR state NOT IN ('READY', 'CONFIRMED') OR next_index = chunk_count
    ),
    CONSTRAINT staged_uploads_declared_md5_check CHECK (declared_md5 IS NULL OR declared_md5 <> ''),
    CONSTRAINT staged_uploads_tmp_key_check CHECK ((visibility = 'PUBLIC') = (tmp_key IS NOT NULL)),
    CONSTRAINT staged_uploads_md5_visibility_check CHECK ((visibility = 'PUBLIC') = (declared_md5 IS NOT NULL)),
    CONSTRAINT staged_uploads_state_check CHECK (
        state IN ('PENDING', 'UPLOADING', 'UPLOADED', 'READY', 'REJECTED', 'CONFIRMED', 'DELETING', 'DELETED')
    ),
    CONSTRAINT staged_uploads_reject_reason_check CHECK (
        reject_reason IS NULL OR reject_reason IN (
            'UNRECOGNIZED_FORMAT', 'SIZE_MISMATCH', 'CHECKSUM_MISMATCH', 'PROCESSING_FAILED', 'EXPIRED'
        )
    ),
    CONSTRAINT staged_uploads_reject_state_check CHECK (
        (state <> 'REJECTED' OR reject_reason IS NOT NULL)
        AND (reject_reason IS NULL OR state IN ('REJECTED', 'DELETING', 'DELETED'))
    ),
    CONSTRAINT staged_uploads_final_key_state_check CHECK (
        state NOT IN ('UPLOADED', 'READY', 'CONFIRMED') OR final_key IS NOT NULL
    ),
    CONSTRAINT staged_uploads_asset_state_check CHECK (
        (state NOT IN ('READY', 'CONFIRMED') OR asset_id IS NOT NULL)
        AND (asset_id IS NULL OR state IN ('READY', 'CONFIRMED', 'DELETING', 'DELETED'))
    )
);
CREATE INDEX staged_uploads_gc ON media.staged_uploads (expires_at) WHERE state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED');
CREATE INDEX staged_uploads_deleting ON media.staged_uploads (id) WHERE state = 'DELETING';
CREATE INDEX staged_uploads_uploading_rewrap ON media.staged_uploads (id) WHERE state = 'UPLOADING';

CREATE TABLE media.secure_chunks (
    asset_id   UUID    NOT NULL,
    idx        INTEGER NOT NULL,
    object_key TEXT    NOT NULL,
    PRIMARY KEY (asset_id, idx),
    CONSTRAINT secure_chunks_object_key_key UNIQUE (object_key),
    CONSTRAINT secure_chunks_idx_check CHECK (idx >= 0),
    CONSTRAINT secure_chunks_object_key_check CHECK (object_key LIKE 'secure/%')
);

CREATE TABLE media.upload_claims (
    upload_id   UUID        PRIMARY KEY,
    kind        TEXT        NOT NULL,
    asset_id    UUID,
    claimed_xid XID8        NOT NULL DEFAULT pg_current_xact_id(),
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT upload_claims_upload_fkey FOREIGN KEY (upload_id) REFERENCES media.staged_uploads (id),
    CONSTRAINT upload_claims_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT upload_claims_asset_key UNIQUE (asset_id),
    CONSTRAINT upload_claims_kind_check CHECK (kind IN ('CONFIRMED', 'RECLAIMED')),
    CONSTRAINT upload_claims_asset_check CHECK (kind <> 'CONFIRMED' OR asset_id IS NOT NULL)
);

CREATE TABLE media.asset_deletion_requests (
    asset_id     UUID        PRIMARY KEY,
    requested_at TIMESTAMPTZ NOT NULL,
    applied_at   TIMESTAMPTZ,
    CONSTRAINT asset_deletion_requests_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id)
);
CREATE INDEX asset_deletion_requests_pending ON media.asset_deletion_requests (requested_at) WHERE applied_at IS NULL;

CREATE TABLE media.asset_gates (
    asset_id   UUID        PRIMARY KEY,
    kind       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT asset_gates_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT asset_gates_kind_check CHECK (kind IN ('HOLD', 'DELETE')),
    CONSTRAINT asset_gates_kind_key UNIQUE (asset_id, kind)
);

CREATE TABLE media.asset_holds (
    hold_id    UUID        PRIMARY KEY,
    asset_id   UUID        NOT NULL,
    gate_kind  TEXT        NOT NULL DEFAULT 'HOLD',
    source     TEXT        NOT NULL,
    source_id  UUID        NOT NULL,
    until      TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT asset_holds_gate_fkey FOREIGN KEY (asset_id, gate_kind) REFERENCES media.asset_gates (asset_id, kind),
    CONSTRAINT asset_holds_gate_kind_check CHECK (gate_kind = 'HOLD'),
    CONSTRAINT asset_holds_source_check CHECK (source <> '')
);
CREATE INDEX asset_holds_asset ON media.asset_holds (asset_id);
CREATE INDEX asset_holds_source ON media.asset_holds (source, source_id);
CREATE INDEX asset_holds_until ON media.asset_holds (until);

CREATE TABLE media.asset_hold_releases (
    hold_id     UUID        PRIMARY KEY,
    released_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT asset_hold_releases_hold_fkey FOREIGN KEY (hold_id) REFERENCES media.asset_holds (hold_id)
);

CREATE TABLE media.asset_copy_requests (
    asset_id        UUID        PRIMARY KEY,
    source_asset_id UUID        NOT NULL,
    requested_at    TIMESTAMPTZ NOT NULL,
    completed_at    TIMESTAMPTZ,
    failed_at       TIMESTAMPTZ,
    CONSTRAINT asset_copy_requests_asset_fkey FOREIGN KEY (asset_id) REFERENCES media.assets (id),
    CONSTRAINT asset_copy_requests_source_fkey FOREIGN KEY (source_asset_id) REFERENCES media.assets (id),
    CONSTRAINT asset_copy_requests_outcome_check CHECK (completed_at IS NULL OR failed_at IS NULL),
    CONSTRAINT asset_copy_requests_self_check CHECK (asset_id <> source_asset_id)
);
CREATE INDEX asset_copy_requests_pending ON media.asset_copy_requests (requested_at) WHERE completed_at IS NULL AND failed_at IS NULL;
