-- =====================================================================
-- V5：Membership Billing & Provider Adapter 共通核心（63）
-- =====================================================================
-- payment-service 只保存 provider-side facts、commercial quote snapshot、
-- command idempotency 與 charge-attempt history。buyer／seller membership、
-- period、entitlement 仍由各自 domain database 擁有；本 migration 不建立
-- 任何跨 database foreign key。

-- -----------------------------------------------------
-- 1. Provider account profile
-- -----------------------------------------------------
CREATE TABLE billing_provider_accounts (
    id              BIGINT PRIMARY KEY,                 -- payment-service TSID
    provider        VARCHAR(32)  NOT NULL,
    account_ref     VARCHAR(128) NOT NULL UNIQUE,       -- controlled alias, not secret
    environment     VARCHAR(16)  NOT NULL,
    credential_ref  VARCHAR(256) NOT NULL,              -- secret-manager/env reference only
    capabilities    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status          VARCHAR(16)  NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_billing_provider_accounts_provider
        CHECK (provider <> ''),
    CONSTRAINT chk_billing_provider_accounts_environment
        CHECK (environment IN ('SANDBOX', 'PRODUCTION')),
    CONSTRAINT chk_billing_provider_accounts_capabilities_object
        CHECK (jsonb_typeof(capabilities) = 'object'),
    CONSTRAINT chk_billing_provider_accounts_status
        CHECK (status IN ('ACTIVE', 'DISABLED'))
);

CREATE INDEX idx_billing_provider_accounts_provider_status
    ON billing_provider_accounts(provider, status);

COMMENT ON TABLE billing_provider_accounts IS
    'Provider account aliases and capability metadata; raw credentials are never stored';
COMMENT ON COLUMN billing_provider_accounts.credential_ref IS
    'Secret manager or environment reference; must not contain raw provider credentials';
COMMENT ON COLUMN billing_provider_accounts.capabilities IS
    'Adapter routing metadata only; never an entitlement or quota source of truth';

CREATE TRIGGER trigger_billing_provider_accounts_updated_at
    BEFORE UPDATE ON billing_provider_accounts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- -----------------------------------------------------
-- 2. Provider subscription lifecycle
-- -----------------------------------------------------
CREATE TABLE membership_provider_subscriptions (
    id                          BIGINT PRIMARY KEY,      -- payment-service TSID
    subject_type                VARCHAR(16)  NOT NULL,
    subject_id                  VARCHAR(64)  NOT NULL,
    membership_id               VARCHAR(64)  NOT NULL,  -- cross-service reference
    membership_change_request_id VARCHAR(64),
    billing_quote_id            VARCHAR(128) NOT NULL,
    subscription_kind           VARCHAR(32)  NOT NULL DEFAULT 'BASE_MEMBERSHIP',
    provider_account_ref        VARCHAR(128) NOT NULL,
    provider                    VARCHAR(32)  NOT NULL,
    provider_customer_id        VARCHAR(128),
    provider_subscription_id    VARCHAR(128),
    subscription_seq            INTEGER      NOT NULL,
    mer_trade_no                VARCHAR(128) NOT NULL UNIQUE,
    offer_id                    VARCHAR(64)  NOT NULL,
    offer_version_no            VARCHAR(32)  NOT NULL,
    quote_snapshot_hash         CHAR(64)     NOT NULL,
    quote_expires_at             TIMESTAMPTZ  NOT NULL,
    currency                    CHAR(3)      NOT NULL,
    unit_amount_minor            BIGINT       NOT NULL,
    interval_unit                VARCHAR(16)  NOT NULL,
    interval_count               SMALLINT     NOT NULL,
    first_period_date            TIMESTAMPTZ,
    provider_anchor_at           TIMESTAMPTZ,
    status                       VARCHAR(16)  NOT NULL DEFAULT 'PENDING',
    created_at                   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_membership_provider_subscriptions_account
        FOREIGN KEY (provider_account_ref)
        REFERENCES billing_provider_accounts(account_ref)
        ON DELETE RESTRICT,
    CONSTRAINT chk_membership_provider_subscriptions_subject_type
        CHECK (subject_type IN ('BUYER', 'SELLER')),
    CONSTRAINT chk_membership_provider_subscriptions_subject_id
        CHECK (subject_id <> ''),
    CONSTRAINT chk_membership_provider_subscriptions_kind
        CHECK (subscription_kind = 'BASE_MEMBERSHIP'),
    CONSTRAINT chk_membership_provider_subscriptions_sequence
        CHECK (subscription_seq > 0),
    CONSTRAINT chk_membership_provider_subscriptions_hash
        CHECK (quote_snapshot_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_membership_provider_subscriptions_amount
        CHECK (unit_amount_minor > 0),
    CONSTRAINT chk_membership_provider_subscriptions_interval
        CHECK (interval_count > 0),
    CONSTRAINT chk_membership_provider_subscriptions_status
        CHECK (status IN ('PENDING', 'ACTIVE', 'SUSPENDED', 'ENDED'))
);

-- A quote is immutable and can create only one provider subscription. Retries
-- must resolve through billing_commands instead of inserting a second row.
CREATE UNIQUE INDEX uq_membership_provider_subscriptions_quote
    ON membership_provider_subscriptions(billing_quote_id);

CREATE UNIQUE INDEX uq_membership_provider_subscriptions_provider_id
    ON membership_provider_subscriptions(provider, provider_subscription_id)
    WHERE provider_subscription_id IS NOT NULL;

-- Sequence is part of the deterministic merchant trade number. Keep it unique per subject so a
-- concurrent retry cannot reuse an old sequence after a previous subscription was ended.
CREATE UNIQUE INDEX uq_membership_provider_subscriptions_subject_sequence
    ON membership_provider_subscriptions(subject_type, subject_id, subscription_kind, subscription_seq);

-- One active base subscription per explicit subject. PENDING is included so a
-- hosted checkout cannot race another checkout into two provider subscriptions.
CREATE UNIQUE INDEX uq_membership_provider_subscriptions_active_base
    ON membership_provider_subscriptions(subject_type, subject_id, subscription_kind)
    WHERE status IN ('PENDING', 'ACTIVE', 'SUSPENDED');

CREATE INDEX idx_membership_provider_subscriptions_subject
    ON membership_provider_subscriptions(subject_type, subject_id, created_at DESC);
CREATE INDEX idx_membership_provider_subscriptions_status
    ON membership_provider_subscriptions(status, updated_at);

COMMENT ON TABLE membership_provider_subscriptions IS
    'Provider subscription lifecycle and immutable BillingQuote audit snapshot';
COMMENT ON COLUMN membership_provider_subscriptions.subject_id IS
    'Buyer buyer_id or seller store_id; never an account_id and no cross-database FK';
COMMENT ON COLUMN membership_provider_subscriptions.unit_amount_minor IS
    'Server-issued offer snapshot in the smallest currency unit; never client supplied';
COMMENT ON COLUMN membership_provider_subscriptions.quote_snapshot_hash IS
    'Canonical UTF-8 JSON SHA-256 over subject, membership, quote, and commercial terms';

CREATE TRIGGER trigger_membership_provider_subscriptions_updated_at
    BEFORE UPDATE ON membership_provider_subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- -----------------------------------------------------
-- 3. Append-only provider charge-attempt ledger
-- -----------------------------------------------------
CREATE TABLE membership_charge_attempts (
    id                           BIGINT PRIMARY KEY,     -- payment-service TSID
    provider_subscription_ref    BIGINT       NOT NULL,
    subject_type                 VARCHAR(16)  NOT NULL,
    subject_id                   VARCHAR(64)  NOT NULL,
    membership_id                VARCHAR(64)  NOT NULL,
    membership_change_request_id VARCHAR(64),
    billing_quote_id             VARCHAR(128),
    quote_snapshot_hash          CHAR(64),
    provider_period_id           VARCHAR(128),
    period_order_no              VARCHAR(128),
    this_period                  INTEGER,
    trade_no                     VARCHAR(128),
    amount_minor                 BIGINT,
    currency                     CHAR(3),
    provider_event_id            VARCHAR(256),
    provider_event_key           VARCHAR(512) NOT NULL,
    status                       VARCHAR(16)  NOT NULL,
    res_code                     VARCHAR(128),
    res_code_msg                 TEXT,
    valid_from                   TIMESTAMPTZ,
    valid_until                  TIMESTAMPTZ,
    cycle_started_at             TIMESTAMPTZ,
    cycle_until                  TIMESTAMPTZ,
    is_initial_authorization     BOOLEAN      NOT NULL DEFAULT FALSE,
    is_authoritative             BOOLEAN      NOT NULL DEFAULT FALSE,
    canonical_event_id           VARCHAR(256),
    received_at                  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_membership_charge_attempts_subscription
        FOREIGN KEY (provider_subscription_ref)
        REFERENCES membership_provider_subscriptions(id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_membership_charge_attempts_subject_type
        CHECK (subject_type IN ('BUYER', 'SELLER')),
    CONSTRAINT chk_membership_charge_attempts_status
        CHECK (status IN ('SUCCEEDED', 'FAILED', 'CONFLICT')),
    CONSTRAINT chk_membership_charge_attempts_amount
        CHECK (amount_minor IS NULL OR amount_minor >= 0),
    CONSTRAINT chk_membership_charge_attempts_hash
        CHECK (quote_snapshot_hash IS NULL OR quote_snapshot_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_membership_charge_attempts_bounds
        CHECK (valid_from IS NULL AND valid_until IS NULL
            OR valid_from IS NOT NULL AND valid_until IS NOT NULL AND valid_from < valid_until)
);

-- Provider notification dedup is exact and independent from canonical event id.
CREATE UNIQUE INDEX uq_membership_charge_attempts_provider_event_key
    ON membership_charge_attempts(provider_event_key);

CREATE UNIQUE INDEX uq_membership_charge_attempts_canonical_event_id
    ON membership_charge_attempts(canonical_event_id)
    WHERE canonical_event_id IS NOT NULL;

-- First-terminal-wins: at most one authoritative result for a provider period.
-- Prefixing the two provider identity namespaces prevents a period ID from
-- colliding with a PeriodOrderNo that happens to have the same text.
CREATE UNIQUE INDEX uq_membership_charge_attempts_authoritative_period
    ON membership_charge_attempts(
        provider_subscription_ref,
        (CASE
            WHEN provider_period_id IS NOT NULL THEN 'PERIOD:' || provider_period_id
            ELSE 'ORDER:' || period_order_no
        END)
    )
    WHERE is_authoritative = TRUE
      AND (provider_period_id IS NOT NULL OR period_order_no IS NOT NULL);

CREATE INDEX idx_membership_charge_attempts_subscription_received
    ON membership_charge_attempts(provider_subscription_ref, received_at DESC);
CREATE INDEX idx_membership_charge_attempts_subject_received
    ON membership_charge_attempts(subject_type, subject_id, received_at DESC);

COMMENT ON TABLE membership_charge_attempts IS
    'Append-only normalized provider result ledger; authoritative winner is never overwritten';
COMMENT ON COLUMN membership_charge_attempts.provider_event_key IS
    'Deterministic provider notification dedup key; distinct from canonical event_id';
COMMENT ON COLUMN membership_charge_attempts.is_authoritative IS
    'First terminal result for a provider subscription period only; conflicts remain audit rows';

-- -----------------------------------------------------
-- 4. Shared billing command idempotency boundary
-- -----------------------------------------------------
CREATE TABLE billing_commands (
    id                           UUID PRIMARY KEY,
    operation                    VARCHAR(64)  NOT NULL,
    subject_type                 VARCHAR(16)  NOT NULL,
    subject_id                   VARCHAR(64)  NOT NULL,
    idempotency_key              VARCHAR(128) NOT NULL,
    request_hash                 CHAR(64)     NOT NULL,
    billing_quote_id             VARCHAR(128),
    result_subscription_id       BIGINT,
    status                       VARCHAR(16)  NOT NULL DEFAULT 'PROCESSING',
    error_code                   VARCHAR(64),
    created_at                   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_billing_commands_subscription
        FOREIGN KEY (result_subscription_id)
        REFERENCES membership_provider_subscriptions(id)
        ON DELETE RESTRICT,
    CONSTRAINT chk_billing_commands_operation
        CHECK (operation IN ('CREATE_SUBSCRIPTION', 'SUSPEND_SUBSCRIPTION',
                             'RESTART_SUBSCRIPTION', 'END_SUBSCRIPTION')),
    CONSTRAINT chk_billing_commands_subject_type
        CHECK (subject_type IN ('BUYER', 'SELLER')),
    CONSTRAINT chk_billing_commands_hash
        CHECK (request_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT chk_billing_commands_status
        CHECK (status IN ('PROCESSING', 'SUCCEEDED', 'FAILED'))
);

CREATE UNIQUE INDEX uq_billing_commands_operation_key
    ON billing_commands(operation, idempotency_key);
CREATE INDEX idx_billing_commands_subject_created
    ON billing_commands(subject_type, subject_id, created_at DESC);
CREATE INDEX idx_billing_commands_processing
    ON billing_commands(status, updated_at)
    WHERE status = 'PROCESSING';

COMMENT ON TABLE billing_commands IS
    'Shared billing RPC/provider action idempotency; same key plus different hash is a conflict';
COMMENT ON COLUMN billing_commands.request_hash IS
    'Canonical request SHA-256; same operation/key with a different hash is rejected';

CREATE TRIGGER trigger_billing_commands_updated_at
    BEFORE UPDATE ON billing_commands
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
