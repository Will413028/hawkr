
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA identity;

CREATE TABLE identity.accounts (
    id            BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         TEXT,
    self_entered_email TEXT,
    suspended_at  TIMESTAMPTZ,
    deleted_at    TIMESTAMPTZ,
    deletion_pending_at TIMESTAMPTZ,
    preferred_locale TEXT      NOT NULL DEFAULT 'zh-TW',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT accounts_preferred_locale_check CHECK (preferred_locale IN ('zh-TW', 'en')),
    CONSTRAINT accounts_deleted_check CHECK (deleted_at IS NULL OR (email IS NULL AND self_entered_email IS NULL AND suspended_at IS NULL)),
    CONSTRAINT accounts_self_entered_email_check CHECK (
        self_entered_email = lower(self_entered_email) AND char_length(self_entered_email) <= 254 AND self_entered_email ~ '^[^@[:space:]]+@[^@[:space:]]+$'
    ),
    CONSTRAINT accounts_deletion_pending_check CHECK (deleted_at IS NULL OR deletion_pending_at IS NULL)
);

CREATE TABLE identity.account_identities (
    provider       TEXT         NOT NULL,
    subject        TEXT         NOT NULL,
    account_id     BIGINT       NOT NULL REFERENCES identity.accounts (id),
    email          TEXT,
    email_verified BOOLEAN      NOT NULL DEFAULT FALSE,
    auto_link_eligible BOOLEAN  NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ  NOT NULL,
    updated_at     TIMESTAMPTZ  NOT NULL,
    PRIMARY KEY (provider, subject),
    CONSTRAINT account_identities_provider_check CHECK (provider::text IN ('GOOGLE', 'LINE')),
    CONSTRAINT account_identities_subject_check CHECK (char_length(subject) >= 1),
    CONSTRAINT account_identities_email_check CHECK (email IS NULL OR char_length(email) >= 1),
    CONSTRAINT account_identities_verified_check CHECK (NOT email_verified OR email IS NOT NULL),
    CONSTRAINT account_identities_auto_link_check CHECK (NOT auto_link_eligible OR (provider = 'GOOGLE' AND email_verified))
);

CREATE INDEX idx_account_identities_account_id ON identity.account_identities (account_id);

CREATE TABLE identity.sessions (
    id                  UUID         PRIMARY KEY DEFAULT uuidv7(),
    token_hash          BYTEA        NOT NULL,
    account_id          BIGINT       NOT NULL REFERENCES identity.accounts (id),
    client              TEXT         NOT NULL,
    created_at          TIMESTAMPTZ  NOT NULL,
    last_used_at        TIMESTAMPTZ  NOT NULL,
    absolute_expires_at TIMESTAMPTZ  NOT NULL,
    auth_time           TIMESTAMPTZ  NOT NULL,
    CONSTRAINT sessions_client_check CHECK (client::text IN ('WEB', 'APP')),
    CONSTRAINT sessions_token_hash_check CHECK (octet_length(token_hash) = 32),
    CONSTRAINT sessions_times_check CHECK (last_used_at >= created_at AND absolute_expires_at > created_at AND auth_time >= created_at)
);

CREATE UNIQUE INDEX uidx_sessions_token_hash ON identity.sessions (token_hash);

CREATE INDEX idx_sessions_account_id ON identity.sessions (account_id);

CREATE TABLE identity.login_flows (
    state_hash       BYTEA        PRIMARY KEY,
    provider         TEXT         NOT NULL,
    code_verifier    TEXT         NOT NULL,
    nonce            TEXT         NOT NULL,
    return_path      TEXT         NOT NULL,
    binding_hash     BYTEA        NOT NULL,
    link_account_id  BIGINT       REFERENCES identity.accounts (id),
    created_at       TIMESTAMPTZ  NOT NULL,
    expires_at       TIMESTAMPTZ  NOT NULL,
    CONSTRAINT login_flows_provider_check CHECK (provider::text IN ('GOOGLE', 'LINE')),
    CONSTRAINT login_flows_state_hash_check CHECK (octet_length(state_hash) = 32),
    CONSTRAINT login_flows_binding_hash_check CHECK (octet_length(binding_hash) = 32),
    CONSTRAINT login_flows_return_path_check CHECK (return_path LIKE '/%' AND return_path NOT LIKE '//%' AND return_path NOT LIKE '/\\%'),
    CONSTRAINT login_flows_expires_check CHECK (expires_at > created_at)
);

CREATE INDEX idx_login_flows_expires_at ON identity.login_flows (expires_at);

CREATE TABLE identity.app_login_nonces (
    nonce_hash  BYTEA        PRIMARY KEY,
    created_at  TIMESTAMPTZ  NOT NULL,
    expires_at  TIMESTAMPTZ  NOT NULL,
    CONSTRAINT app_login_nonces_nonce_hash_check CHECK (octet_length(nonce_hash) = 32),
    CONSTRAINT app_login_nonces_expires_check CHECK (expires_at > created_at)
);

CREATE INDEX idx_app_login_nonces_expires_at ON identity.app_login_nonces (expires_at);

CREATE TABLE identity.account_suspensions (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    account_id      BIGINT      NOT NULL REFERENCES identity.accounts (id),
    kind            TEXT        NOT NULL DEFAULT 'PERMANENT',
    reason_category TEXT        NOT NULL,
    reason_text     TEXT        NOT NULL DEFAULT '',
    source_ref      TEXT,
    suspended_at    TIMESTAMPTZ NOT NULL,
    released_at     TIMESTAMPTZ,
    release_reason  TEXT,
    CONSTRAINT account_suspensions_kind_check CHECK (kind::text IN ('PERMANENT')),
    CONSTRAINT account_suspensions_reason_category_check CHECK (reason_category::text IN ('FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'OTHER', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT')),
    CONSTRAINT account_suspensions_reason_text_check CHECK (char_length(reason_text) <= 1000 AND reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT account_suspensions_source_ref_check CHECK (source_ref IS NULL OR (char_length(source_ref) >= 1 AND char_length(source_ref) <= 200 AND source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')),
    CONSTRAINT account_suspensions_release_check CHECK (
        (released_at IS NULL AND release_reason IS NULL)
        OR (released_at IS NOT NULL AND release_reason IS NOT NULL AND released_at >= suspended_at)
    ),
    CONSTRAINT account_suspensions_release_reason_check CHECK (release_reason IS NULL OR (char_length(release_reason) >= 1 AND char_length(release_reason) <= 1000 AND release_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);

CREATE UNIQUE INDEX uidx_account_suspensions_active ON identity.account_suspensions (account_id) WHERE released_at IS NULL;

CREATE INDEX idx_account_suspensions_account_id_suspended_at ON identity.account_suspensions (account_id, suspended_at DESC);

CREATE TABLE identity.account_roles (
    account_id BIGINT       NOT NULL REFERENCES identity.accounts (id),
    role       TEXT         NOT NULL,
    granted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, role),
    CONSTRAINT account_roles_role_check CHECK (role::text IN ('SELLER'))
);

CREATE TABLE identity.admins (
    id                 BIGINT       GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    access_subject     TEXT         NOT NULL,
    email              TEXT         NOT NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uidx_admins_access_subject ON identity.admins (access_subject);

CREATE TABLE identity.admin_roles (
    admin_id   BIGINT       NOT NULL REFERENCES identity.admins (id),
    role       TEXT         NOT NULL,
    granted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (admin_id, role),
    CONSTRAINT admin_roles_role_check CHECK (role::text IN ('OWNER'))
);

CREATE TABLE identity.admin_role_events (
    id          UUID         PRIMARY KEY DEFAULT uuidv7(),
    admin_id    BIGINT       NOT NULL REFERENCES identity.admins (id),
    role        TEXT         NOT NULL,
    action      TEXT         NOT NULL,
    reason      TEXT         NOT NULL,
    source      TEXT         NOT NULL,
    db_user     TEXT         NOT NULL,
    os_user     TEXT,
    occurred_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT admin_role_events_role_check CHECK (role::text IN ('OWNER')),
    CONSTRAINT admin_role_events_action_check CHECK (action::text IN ('GRANT', 'REVOKE')),
    CONSTRAINT admin_role_events_reason_check CHECK (char_length(reason) >= 1 AND char_length(reason) <= 1000 AND reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT admin_role_events_source_check CHECK (source::text IN ('CLI'))
);

CREATE INDEX idx_admin_role_events_admin_id_occurred_at ON identity.admin_role_events (admin_id, occurred_at DESC);
