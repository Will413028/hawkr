
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA delegation;

CREATE TABLE delegation.admin_eligibilities (
    admin_id BIGINT PRIMARY KEY
);


CREATE TABLE delegation.delegated_operation_sessions (
    id                       BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    display_code             TEXT         NOT NULL,
    credential_hash          BYTEA,
    handoff_code_hash        BYTEA,
    handoff_expires_at       TIMESTAMPTZ,
    actor_admin_id           BIGINT       NOT NULL,
    actor_display_name       TEXT         NOT NULL,
    target_store_id          BIGINT       NOT NULL,
    target_seller_account_id BIGINT       NOT NULL,
    target_store_name        TEXT         NOT NULL,
    reason_code              TEXT         NOT NULL,
    reason_label             TEXT         NOT NULL,
    case_reference           TEXT,
    capabilities             TEXT[]       NOT NULL,
    issued_at                TIMESTAMPTZ  NOT NULL,
    expires_at               TIMESTAMPTZ  NOT NULL,
    last_active_at           TIMESTAMPTZ  NOT NULL,
    ended_at                 TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ,
    revoked_by_admin_id      BIGINT,
    revoke_reason            TEXT,
    status                   TEXT         NOT NULL,
    revision                 INTEGER      NOT NULL DEFAULT 0,
    idempotency_key          TEXT         NOT NULL,
    create_request_id        TEXT         NOT NULL,
    create_correlation_id    TEXT         NOT NULL,
    create_user_agent        TEXT,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT delegated_operation_sessions_status_check CHECK (status::text IN ('ACTIVE', 'REVOKED')),
    CONSTRAINT delegated_operation_sessions_reason_code_check CHECK (
        reason_code::text IN ('SELLER_REQUEST', 'PLATFORM_MAINTENANCE', 'CONTENT_GOVERNANCE')
    ),
    CONSTRAINT delegated_operation_sessions_lifetime_check CHECK (
        expires_at > issued_at AND expires_at <= issued_at + INTERVAL '1800 seconds'
    ),
    CONSTRAINT delegated_operation_sessions_second_precision_check CHECK (
        issued_at = date_trunc('second', issued_at)
        AND expires_at = date_trunc('second', expires_at)
        AND last_active_at = date_trunc('second', last_active_at)
    ),
    CONSTRAINT delegated_operation_sessions_last_active_check CHECK (
        last_active_at >= issued_at AND last_active_at <= expires_at
    ),
    CONSTRAINT delegated_operation_sessions_revision_check CHECK (revision >= 0),
    CONSTRAINT delegated_operation_sessions_revoked_check CHECK ((status = 'REVOKED') = (revoked_at IS NOT NULL)),
    CONSTRAINT delegated_operation_sessions_revoke_reason_check CHECK (
        revoke_reason IS NULL OR char_length(revoke_reason) BETWEEN 1 AND 200
    ),
    CONSTRAINT delegated_operation_sessions_handoff_check CHECK (
        handoff_code_hash IS NULL OR handoff_expires_at IS NOT NULL
    ),
    CONSTRAINT delegated_operation_sessions_case_reference_check CHECK (
        case_reference IS NULL
        OR (char_length(case_reference) >= 4 AND char_length(case_reference) <= 200 AND case_reference ~ '[0-9]')
    ),
    CONSTRAINT delegated_operation_sessions_display_code_check CHECK (display_code ~ '^OPS-[0-9A-Z]{13}$'),
    CONSTRAINT delegated_operation_sessions_capabilities_check CHECK (
        cardinality(capabilities) >= 1 AND cardinality(capabilities) <= 10
        AND capabilities <@ ARRAY[
              'storefront.read', 'storefront.write',
              'products.read', 'products.write', 'products.publish',
              'posts.read', 'posts.write', 'posts.publish',
              'orders.read', 'media.upload'
            ]::text[]
        AND cardinality(capabilities) =
              (CASE WHEN 'storefront.read'  = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'storefront.write' = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.read'    = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.write'   = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.publish' = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.read'       = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.write'      = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.publish'    = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'orders.read'      = ANY (capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'media.upload'     = ANY (capabilities) THEN 1 ELSE 0 END)
    )
);

CREATE UNIQUE INDEX uidx_delegated_operation_sessions_actor_idempotency
    ON delegation.delegated_operation_sessions (actor_admin_id, idempotency_key);
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_credential_hash
    ON delegation.delegated_operation_sessions (credential_hash);
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_handoff_code_hash
    ON delegation.delegated_operation_sessions (handoff_code_hash) WHERE handoff_code_hash IS NOT NULL;
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_display_code
    ON delegation.delegated_operation_sessions (display_code);
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_active_actor
    ON delegation.delegated_operation_sessions (actor_admin_id) WHERE status = 'ACTIVE' AND ended_at IS NULL;
CREATE UNIQUE INDEX uidx_delegated_operation_sessions_active_store
    ON delegation.delegated_operation_sessions (target_store_id) WHERE status = 'ACTIVE' AND ended_at IS NULL;
CREATE INDEX idx_delegated_operation_sessions_actor_created_at
    ON delegation.delegated_operation_sessions (actor_admin_id, created_at DESC);
CREATE INDEX idx_delegated_operation_sessions_store_created_at
    ON delegation.delegated_operation_sessions (target_store_id, created_at DESC);

CREATE TABLE delegation.seller_delegation_authorizations (
    id                   UUID         PRIMARY KEY,
    target_store_id      BIGINT,
    source               TEXT         NOT NULL,
    source_reference     TEXT         NOT NULL,
    granted_capabilities TEXT[]       NOT NULL,
    effective_from       TIMESTAMPTZ  NOT NULL,
    effective_until      TIMESTAMPTZ,
    approved_by_admin_id BIGINT       NOT NULL,
    approved_at          TIMESTAMPTZ  NOT NULL,
    revoked_at           TIMESTAMPTZ,
    revoked_by_admin_id  BIGINT,
    revoke_reason        TEXT,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT seller_delegation_authorizations_source_check CHECK (
        source::text IN ('CONTRACT', 'TICKET', 'PLATFORM_POLICY')
    ),
    CONSTRAINT seller_delegation_authorizations_global_check CHECK (
        target_store_id IS NOT NULL OR source = 'PLATFORM_POLICY'
    ),
    CONSTRAINT seller_delegation_authorizations_source_reference_check CHECK (
        char_length(source_reference) >= 4 AND char_length(source_reference) <= 200 AND source_reference ~ '[0-9]'
    ),
    CONSTRAINT seller_delegation_authorizations_period_check CHECK (
        effective_until IS NULL OR effective_until > effective_from
    ),
    CONSTRAINT seller_delegation_authorizations_revoked_check CHECK (
        (revoked_at IS NULL) = (revoked_by_admin_id IS NULL) AND (revoked_at IS NULL) = (revoke_reason IS NULL)
    ),
    CONSTRAINT seller_delegation_authorizations_revoke_reason_check CHECK (
        revoke_reason IS NULL OR char_length(revoke_reason) BETWEEN 1 AND 200
    ),
    CONSTRAINT seller_delegation_authorizations_capabilities_check CHECK (
        cardinality(granted_capabilities) >= 1 AND cardinality(granted_capabilities) <= 10
        AND granted_capabilities <@ ARRAY[
              'storefront.read', 'storefront.write',
              'products.read', 'products.write', 'products.publish',
              'posts.read', 'posts.write', 'posts.publish',
              'orders.read', 'media.upload'
            ]::text[]
        AND cardinality(granted_capabilities) =
              (CASE WHEN 'storefront.read'  = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'storefront.write' = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.read'    = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.write'   = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'products.publish' = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.read'       = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.write'      = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'posts.publish'    = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'orders.read'      = ANY (granted_capabilities) THEN 1 ELSE 0 END)
            + (CASE WHEN 'media.upload'     = ANY (granted_capabilities) THEN 1 ELSE 0 END)
    )
);

CREATE UNIQUE INDEX uidx_seller_delegation_authorizations_store
    ON delegation.seller_delegation_authorizations (target_store_id) WHERE target_store_id IS NOT NULL;
CREATE UNIQUE INDEX uidx_seller_delegation_authorizations_global
    ON delegation.seller_delegation_authorizations ((target_store_id IS NULL)) WHERE target_store_id IS NULL;

CREATE TABLE delegation.admin_operation_audit_logs (
    id                       UUID         PRIMARY KEY,
    session_id               BIGINT       NOT NULL REFERENCES delegation.delegated_operation_sessions (id),
    actor_admin_id           BIGINT       NOT NULL,
    actor_display_name       TEXT         NOT NULL,
    target_store_id          BIGINT       NOT NULL,
    target_seller_account_id BIGINT       NOT NULL,
    target_store_name        TEXT         NOT NULL,
    origin                   TEXT         NOT NULL,
    capability               TEXT         NOT NULL,
    action                   TEXT         NOT NULL,
    resource_type            TEXT         NOT NULL,
    resource_id              TEXT,
    resource_name            TEXT,
    result                   TEXT         NOT NULL,
    biz_code                 INTEGER,
    failure_reason           TEXT,
    operation_reason         TEXT,
    case_reference           TEXT,
    request_id               TEXT         NOT NULL,
    correlation_id           TEXT         NOT NULL,
    idempotency_key          TEXT,
    payload_hash             TEXT,
    before_snapshot          JSONB,
    after_snapshot           JSONB,
    changed_fields           JSONB,
    resource_version_before  BIGINT,
    resource_version_after   BIGINT,
    user_agent               TEXT,
    risk_level               TEXT         NOT NULL,
    notify_seller            BOOLEAN      NOT NULL,
    notified_at              TIMESTAMPTZ,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    confirmed_at             TIMESTAMPTZ,
    CONSTRAINT admin_operation_audit_logs_result_check CHECK (result::text IN ('PENDING', 'SUCCEEDED', 'FAILED')),
    CONSTRAINT admin_operation_audit_logs_confirmed_check CHECK ((result = 'PENDING') = (confirmed_at IS NULL)),
    CONSTRAINT admin_operation_audit_logs_biz_code_check CHECK (result = 'FAILED' OR biz_code IS NULL),
    CONSTRAINT admin_operation_audit_logs_failure_reason_check CHECK (
        (result = 'FAILED') = (failure_reason IS NOT NULL)
        AND (failure_reason IS NULL OR (failure_reason ~ '^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$' AND failure_reason <> 'UNSPECIFIED'))
    ),
    CONSTRAINT admin_operation_audit_logs_notified_check CHECK (
        notified_at IS NULL OR (notify_seller AND result = 'SUCCEEDED')
    ),
    CONSTRAINT admin_operation_audit_logs_risk_level_check CHECK (risk_level::text IN ('LOW', 'MEDIUM', 'HIGH')),
    CONSTRAINT admin_operation_audit_logs_resource_type_check CHECK (
        resource_type::text IN ('STOREFRONT', 'PRODUCT', 'POST', 'ORDER', 'MEDIA')
    ),
    CONSTRAINT admin_operation_audit_logs_origin_check CHECK (origin::text IN ('INTERACTIVE', 'SCHEDULED')),
    CONSTRAINT admin_operation_audit_logs_reason_check CHECK (
        operation_reason IS NULL OR char_length(operation_reason) BETWEEN 1 AND 100
    ),
    CONSTRAINT admin_operation_audit_logs_action_check CHECK (
        action::text IN (
            'STOREFRONT_UPDATE', 'STOREFRONT_LOGO_UPDATE', 'STOREFRONT_COVER_UPDATE',
            'PRODUCT_CREATE', 'PRODUCT_UPDATE', 'PRODUCT_PUBLISH', 'PRODUCT_UNPUBLISH', 'PRODUCT_BATCH_UNPUBLISH',
            'POST_CREATE', 'POST_UPDATE', 'POST_PUBLISH', 'POST_UNPUBLISH', 'POST_SCHEDULE',
            'ORDER_PII_REVEAL', 'STOREFRONT_READ', 'PRODUCT_READ', 'POST_READ', 'ORDER_READ', 'AUDIT_CORRECTION', 'MEDIA_UPLOAD'
        )
    ),
    CONSTRAINT admin_operation_audit_logs_capability_check CHECK (
        capability::text IN (
            'storefront.read', 'storefront.write',
            'products.read', 'products.write', 'products.publish',
            'posts.read', 'posts.write', 'posts.publish',
            'orders.read', 'media.upload'
        )
    )
);

CREATE INDEX idx_admin_operation_audit_logs_session_created_at
    ON delegation.admin_operation_audit_logs (session_id, created_at DESC);
CREATE INDEX idx_admin_operation_audit_logs_pending
    ON delegation.admin_operation_audit_logs (created_at) WHERE result = 'PENDING';
CREATE UNIQUE INDEX uidx_admin_operation_audit_logs_idempotency
    ON delegation.admin_operation_audit_logs (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_admin_operation_audit_logs_store_created_at
    ON delegation.admin_operation_audit_logs (target_store_id, created_at DESC);
CREATE INDEX idx_admin_operation_audit_logs_actor_created_at
    ON delegation.admin_operation_audit_logs (actor_admin_id, created_at DESC);

CREATE TABLE delegation.admin_operation_audit_log_items (
    audit_log_id            UUID   NOT NULL REFERENCES delegation.admin_operation_audit_logs (id),
    resource_id             TEXT   NOT NULL,
    resource_name           TEXT,
    resource_version_before BIGINT,
    resource_version_after  BIGINT,
    PRIMARY KEY (audit_log_id, resource_id)
);

CREATE TABLE delegation.delegation_authorization_audit_logs (
    id                     UUID         PRIMARY KEY,
    authorization_id       UUID         NOT NULL REFERENCES delegation.seller_delegation_authorizations (id),
    target_store_id        BIGINT       NOT NULL,
    action                 TEXT         NOT NULL,
    actor_admin_id         BIGINT       NOT NULL,
    actor_display_name     TEXT         NOT NULL,
    operation_reason       TEXT         NOT NULL,
    source                 TEXT,
    source_reference       TEXT,
    capabilities_before    TEXT[],
    capabilities_after     TEXT[],
    effective_until_before TIMESTAMPTZ,
    effective_until_after  TIMESTAMPTZ,
    idempotency_key        TEXT,
    request_id             TEXT,
    correlation_id         TEXT,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT delegation_authorization_audit_logs_action_check CHECK (
        action::text IN ('GRANT', 'REINSTATE', 'UPDATE', 'REVOKE')
    ),
    CONSTRAINT delegation_authorization_audit_logs_reason_check CHECK (
        char_length(operation_reason) BETWEEN 1 AND 200
    )
);

CREATE INDEX idx_delegation_authorization_audit_logs_store_created_at
    ON delegation.delegation_authorization_audit_logs (target_store_id, created_at DESC);
CREATE UNIQUE INDEX uidx_delegation_authorization_audit_logs_idempotency
    ON delegation.delegation_authorization_audit_logs (authorization_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
