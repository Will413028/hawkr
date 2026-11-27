
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA payment;

CREATE TABLE payment.accounts (
    store_id               BIGINT      PRIMARY KEY,
    provider               TEXT        NOT NULL,
    external_account_id    TEXT,
    requirement_collection TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'NOT_CREATED',
    transfers_active       BOOLEAN     NOT NULL DEFAULT false,
    payouts_active         BOOLEAN     NOT NULL DEFAULT false,
    requirements           JSONB       NOT NULL DEFAULT '{}',
    status_reason          TEXT,
    fetched_at             TIMESTAMPTZ,
    create_failed_at       TIMESTAMPTZ,
    create_refused_at      TIMESTAMPTZ,
    create_called_at       TIMESTAMPTZ,
    create_completed_at    TIMESTAMPTZ,
    create_idempotency_key UUID        NOT NULL DEFAULT gen_random_uuid(),
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    seller_read_claimed_at TIMESTAMPTZ,
    purge_pending_at       TIMESTAMPTZ,
    purge_blocked_at       TIMESTAMPTZ,
    purge_blocked_reason   TEXT,
    purge_replay_requested_at TIMESTAMPTZ,
    requirements_deadline  TIMESTAMPTZ,
    first_activated_at     TIMESTAMPTZ,
    CONSTRAINT accounts_store_id_check CHECK (store_id > 0),
    CONSTRAINT accounts_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT accounts_external_id_check CHECK (external_account_id IS NULL OR char_length(external_account_id) BETWEEN 1 AND 255),
    CONSTRAINT accounts_collection_check CHECK (requirement_collection IN ('PROVIDER', 'PLATFORM')),
    CONSTRAINT accounts_status_check CHECK (status IN ('NOT_CREATED', 'UNDER_REVIEW', 'ACTION_REQUIRED', 'ENABLED', 'DISABLED')),
    CONSTRAINT accounts_created_check CHECK ((status = 'NOT_CREATED') = (create_completed_at IS NULL)),
    CONSTRAINT accounts_enabled_check CHECK ((status = 'ENABLED') = transfers_active),
    CONSTRAINT accounts_snapshot_check CHECK (create_completed_at IS NOT NULL OR (fetched_at IS NULL AND NOT payouts_active AND status_reason IS NULL)),
    CONSTRAINT accounts_requirements_check CHECK (jsonb_typeof(requirements) = 'object'),
    CONSTRAINT accounts_first_activated_check CHECK (NOT transfers_active OR first_activated_at IS NOT NULL),
    CONSTRAINT accounts_deadline_snapshot_check CHECK (create_completed_at IS NOT NULL OR (requirements_deadline IS NULL AND first_activated_at IS NULL)),
    CONSTRAINT accounts_status_reason_check CHECK (status_reason IS NULL OR char_length(status_reason) BETWEEN 1 AND 200),
    CONSTRAINT accounts_create_failed_check CHECK (create_failed_at IS NULL OR create_completed_at IS NULL),
    CONSTRAINT accounts_create_refused_check CHECK (create_refused_at IS NULL OR create_failed_at IS NOT NULL),
    CONSTRAINT accounts_completed_check CHECK (create_completed_at IS NULL OR external_account_id IS NOT NULL),
    CONSTRAINT accounts_called_check CHECK (external_account_id IS NULL OR create_called_at IS NOT NULL),
    CONSTRAINT accounts_purge_blocked_check CHECK ((purge_blocked_at IS NULL) = (purge_blocked_reason IS NULL) AND (purge_blocked_at IS NULL OR purge_pending_at IS NOT NULL)),
    CONSTRAINT accounts_purge_replay_check CHECK (purge_replay_requested_at IS NULL OR purge_blocked_at IS NOT NULL),
    CONSTRAINT accounts_purge_blocked_reason_check CHECK (purge_blocked_reason IS NULL OR char_length(purge_blocked_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX accounts_external ON payment.accounts (provider, external_account_id) WHERE external_account_id IS NOT NULL;
CREATE INDEX accounts_poll ON payment.accounts (store_id) WHERE status IN ('UNDER_REVIEW', 'ACTION_REQUIRED', 'DISABLED');
CREATE INDEX accounts_create_failed ON payment.accounts (create_failed_at) WHERE create_failed_at IS NOT NULL;
CREATE INDEX accounts_purge_pending ON payment.accounts (store_id) WHERE purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL;
CREATE INDEX accounts_unresolved ON payment.accounts (store_id) WHERE external_account_id IS NULL AND create_called_at IS NOT NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL AND create_refused_at IS NULL;
CREATE INDEX accounts_purge_blocked ON payment.accounts (purge_blocked_at) WHERE purge_blocked_at IS NOT NULL;

CREATE TABLE payment.payments (
    id                     UUID        PRIMARY KEY,
    buyer_id               BIGINT      NOT NULL,
    amount                 BIGINT      NOT NULL,
    provider               TEXT        NOT NULL,
    description            TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'CREATING',
    external_payment_id    TEXT,
    external_charge_id     TEXT,
    paid_at                TIMESTAMPTZ,
    last_error             TEXT,
    failure_reason         TEXT,
    fetched_at             TIMESTAMPTZ,
    create_idempotency_key UUID        NOT NULL DEFAULT gen_random_uuid(),
    create_called_at       TIMESTAMPTZ,
    create_lookup_failures INTEGER     NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT payments_lookup_failures_check CHECK (create_lookup_failures >= 0),
    CONSTRAINT payments_buyer_id_check CHECK (buyer_id > 0),
    CONSTRAINT payments_amount_check CHECK (amount > 0),
    CONSTRAINT payments_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT payments_description_check CHECK (char_length(description) BETWEEN 1 AND 200),
    CONSTRAINT payments_status_check CHECK (status IN ('CREATING', 'AWAITING_PAYMENT', 'PROCESSING', 'SUCCEEDED', 'CANCELED', 'CREATE_FAILED', 'ANOMALY')),
    CONSTRAINT payments_external_id_check CHECK (external_payment_id IS NULL OR char_length(external_payment_id) BETWEEN 1 AND 255),
    CONSTRAINT payments_charge_id_check CHECK (external_charge_id IS NULL OR char_length(external_charge_id) BETWEEN 1 AND 255),
    CONSTRAINT payments_created_check CHECK (status NOT IN ('AWAITING_PAYMENT', 'PROCESSING', 'SUCCEEDED') OR external_payment_id IS NOT NULL),
    CONSTRAINT payments_creating_check CHECK (status <> 'CREATING' OR external_payment_id IS NULL),
    CONSTRAINT payments_called_check CHECK (external_payment_id IS NULL OR create_called_at IS NOT NULL),
    CONSTRAINT payments_paid_check CHECK ((status = 'SUCCEEDED') = (paid_at IS NOT NULL) AND (status <> 'SUCCEEDED' OR external_charge_id IS NOT NULL)),
    CONSTRAINT payments_charge_only_when_paid_check CHECK (status = 'SUCCEEDED' OR external_charge_id IS NULL),
    CONSTRAINT payments_failure_check CHECK ((status IN ('CREATE_FAILED', 'ANOMALY')) = (failure_reason IS NOT NULL)),
    CONSTRAINT payments_failure_reason_check CHECK (failure_reason IS NULL OR char_length(failure_reason) BETWEEN 1 AND 200),
    CONSTRAINT payments_last_error_check CHECK (last_error IS NULL OR char_length(last_error) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX payments_external ON payment.payments (provider, external_payment_id) WHERE external_payment_id IS NOT NULL;
CREATE INDEX payments_creating ON payment.payments (created_at) WHERE status = 'CREATING';
CREATE INDEX payments_open ON payment.payments (id) WHERE status IN ('AWAITING_PAYMENT', 'PROCESSING');

CREATE TABLE payment.payment_orders (
    id         UUID        PRIMARY KEY,
    payment_id UUID        NOT NULL REFERENCES payment.payments (id),
    order_no   TEXT        NOT NULL,
    store_id   BIGINT      NOT NULL,
    amount     BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_orders_order_no_check CHECK (char_length(order_no) BETWEEN 1 AND 64),
    CONSTRAINT payment_orders_store_id_check CHECK (store_id > 0),
    CONSTRAINT payment_orders_amount_check CHECK (amount > 0)
);
CREATE UNIQUE INDEX payment_orders_order ON payment.payment_orders (order_no);
CREATE INDEX payment_orders_payment ON payment.payment_orders (payment_id);

CREATE TABLE payment.payment_attempts (
    id                  UUID        PRIMARY KEY,
    payment_id          UUID        NOT NULL REFERENCES payment.payments (id),
    provider            TEXT        NOT NULL,
    external_payment_id TEXT,
    idempotency_key     UUID        NOT NULL,
    called_at           TIMESTAMPTZ,
    state               TEXT,
    fetched_at          TIMESTAMPTZ,
    superseded_at       TIMESTAMPTZ,
    excess_paid_at      TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT payment_attempts_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'),
    CONSTRAINT payment_attempts_external_id_check CHECK (external_payment_id IS NULL OR (char_length(external_payment_id) >= 1 AND char_length(external_payment_id) <= 255)),
    CONSTRAINT payment_attempts_state_check CHECK (state IS NULL OR state IN ('AWAITING_PAYMENT', 'PROCESSING', 'SUCCEEDED', 'CANCELED')),
    CONSTRAINT payment_attempts_called_check CHECK (external_payment_id IS NULL OR called_at IS NOT NULL),
    CONSTRAINT payment_attempts_state_external_check CHECK (state IS NULL OR external_payment_id IS NOT NULL),
    CONSTRAINT payment_attempts_excess_check CHECK (excess_paid_at IS NULL OR (state = 'SUCCEEDED' AND superseded_at IS NOT NULL))
);
CREATE UNIQUE INDEX payment_attempts_effective ON payment.payment_attempts (payment_id) WHERE superseded_at IS NULL;
CREATE UNIQUE INDEX payment_attempts_external ON payment.payment_attempts (provider, external_payment_id) WHERE external_payment_id IS NOT NULL;
CREATE UNIQUE INDEX payment_attempts_key ON payment.payment_attempts (idempotency_key);
CREATE INDEX payment_attempts_payment ON payment.payment_attempts (payment_id);
CREATE INDEX payment_attempts_excess ON payment.payment_attempts (excess_paid_at) WHERE excess_paid_at IS NOT NULL;

CREATE TABLE payment.transfers (
    id                     UUID        PRIMARY KEY,
    payment_order_id       UUID        NOT NULL REFERENCES payment.payment_orders (id),
    store_id               BIGINT      NOT NULL,
    amount                 BIGINT      NOT NULL,
    reversed_amount        BIGINT      NOT NULL DEFAULT 0,
    destination_account_id TEXT        NOT NULL,
    status                 TEXT        NOT NULL DEFAULT 'PENDING',
    external_transfer_id   TEXT,
    failure_reason         TEXT,
    idempotency_key        UUID        NOT NULL DEFAULT gen_random_uuid(),
    called_at              TIMESTAMPTZ,
    lookup_failures        INTEGER     NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT transfers_lookup_failures_check CHECK (lookup_failures >= 0),
    CONSTRAINT transfers_store_id_check CHECK (store_id > 0),
    CONSTRAINT transfers_amount_check CHECK (amount > 0),
    CONSTRAINT transfers_reversed_check CHECK (reversed_amount BETWEEN 0 AND amount),
    CONSTRAINT transfers_destination_check CHECK (char_length(destination_account_id) BETWEEN 1 AND 255),
    CONSTRAINT transfers_status_check CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED', 'ANOMALY')),
    CONSTRAINT transfers_external_id_check CHECK (external_transfer_id IS NULL OR char_length(external_transfer_id) BETWEEN 1 AND 255),
    CONSTRAINT transfers_succeeded_check CHECK (status <> 'SUCCEEDED' OR external_transfer_id IS NOT NULL),
    CONSTRAINT transfers_external_state_check CHECK (external_transfer_id IS NULL OR status IN ('SUCCEEDED', 'FAILED')),
    CONSTRAINT transfers_reversal_only_when_succeeded_check CHECK (status = 'SUCCEEDED' OR reversed_amount = 0),
    CONSTRAINT transfers_called_check CHECK (external_transfer_id IS NULL OR called_at IS NOT NULL),
    CONSTRAINT transfers_failure_check CHECK ((status IN ('FAILED', 'ANOMALY')) = (failure_reason IS NOT NULL)),
    CONSTRAINT transfers_failure_reason_check CHECK (failure_reason IS NULL OR char_length(failure_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX transfers_active_order ON payment.transfers (payment_order_id) WHERE status IN ('PENDING', 'SUCCEEDED', 'ANOMALY');
CREATE UNIQUE INDEX transfers_external ON payment.transfers (external_transfer_id) WHERE external_transfer_id IS NOT NULL;
CREATE INDEX transfers_pending ON payment.transfers (updated_at, id) WHERE status = 'PENDING';

CREATE TABLE payment.refunds (
    id                       UUID        PRIMARY KEY,
    payment_order_id         UUID        NOT NULL REFERENCES payment.payment_orders (id),
    request_key              TEXT        NOT NULL,
    amount                   BIGINT      NOT NULL,
    auto                     BOOLEAN     NOT NULL DEFAULT false,
    status                   TEXT        NOT NULL DEFAULT 'PENDING',
    reversal_status          TEXT        NOT NULL DEFAULT 'NONE',
    reversal_amount          BIGINT      NOT NULL DEFAULT 0,
    reversal_recovered       BIGINT      NOT NULL DEFAULT 0,
    reversal_reason          TEXT,
    transfer_deducted        BOOLEAN     NOT NULL DEFAULT false,
    topup_status             TEXT        NOT NULL DEFAULT 'NONE',
    topup_reason             TEXT,
    topup_idempotency_key    UUID        NOT NULL DEFAULT gen_random_uuid(),
    topup_called_at          TIMESTAMPTZ,
    topup_external_id        TEXT,
    topup_lookup_failures    INTEGER     NOT NULL DEFAULT 0,
    topup_reversed_amount    BIGINT      NOT NULL DEFAULT 0,
    idempotency_key          UUID        NOT NULL DEFAULT gen_random_uuid(),
    refund_called_at         TIMESTAMPTZ,
    refund_lookup_failures   INTEGER     NOT NULL DEFAULT 0,
    external_refund_id       TEXT,
    failure_reason           TEXT,
    created_at               TIMESTAMPTZ NOT NULL,
    updated_at               TIMESTAMPTZ NOT NULL,
    CONSTRAINT refunds_request_key_check CHECK (char_length(request_key) BETWEEN 1 AND 128),
    CONSTRAINT refunds_amount_check CHECK (amount > 0),
    CONSTRAINT refunds_reversal_amount_check CHECK (reversal_amount BETWEEN 0 AND amount),
    CONSTRAINT refunds_status_check CHECK (status IN ('PENDING', 'REFUNDING', 'SUCCEEDED', 'FAILED', 'ANOMALY')),
    CONSTRAINT refunds_reversal_status_check CHECK (reversal_status IN ('NONE', 'WAITING', 'REVERSING', 'REVERSED', 'RELEASED', 'ANOMALY')),
    CONSTRAINT refunds_reversal_none_check CHECK ((reversal_status = 'NONE') = (reversal_amount = 0)),
    CONSTRAINT refunds_reversal_reason_check CHECK ((reversal_status IN ('RELEASED', 'ANOMALY')) = (reversal_reason IS NOT NULL) AND (reversal_reason IS NULL OR char_length(reversal_reason) BETWEEN 1 AND 200)),
    CONSTRAINT refunds_reversal_recovered_check CHECK (reversal_recovered >= 0 AND reversal_recovered <= reversal_amount),
    CONSTRAINT refunds_reversal_reversed_check CHECK (reversal_status <> 'REVERSED' OR reversal_recovered = reversal_amount),
    CONSTRAINT refunds_reversal_not_started_check CHECK (reversal_status NOT IN ('NONE', 'WAITING') OR reversal_recovered = 0),
    CONSTRAINT refunds_reversal_after_acceptance_check CHECK (reversal_status NOT IN ('REVERSING', 'REVERSED') OR external_refund_id IS NOT NULL),
    CONSTRAINT refunds_failed_releases_waiting_check CHECK (status <> 'FAILED' OR reversal_status <> 'WAITING'),
    CONSTRAINT refunds_topup_status_check CHECK (topup_status IN ('NONE', 'WAITING', 'SENDING', 'DONE', 'RELEASED', 'ANOMALY')),
    CONSTRAINT refunds_topup_only_failed_check CHECK (topup_status = 'NONE' OR status = 'FAILED'),
    CONSTRAINT refunds_topup_reason_check CHECK ((topup_status IN ('RELEASED', 'ANOMALY')) = (topup_reason IS NOT NULL) AND (topup_reason IS NULL OR char_length(topup_reason) BETWEEN 1 AND 200)),
    CONSTRAINT refunds_topup_external_check CHECK ((topup_status = 'DONE') = (topup_external_id IS NOT NULL) AND (topup_external_id IS NULL OR char_length(topup_external_id) BETWEEN 1 AND 255)),
    CONSTRAINT refunds_topup_called_check CHECK (topup_external_id IS NULL OR topup_called_at IS NOT NULL),
    CONSTRAINT refunds_topup_lookup_failures_check CHECK (topup_lookup_failures >= 0),
    CONSTRAINT refunds_topup_reversed_check CHECK (topup_reversed_amount >= 0 AND topup_reversed_amount <= amount),
    CONSTRAINT refunds_topup_reversed_only_done_check CHECK (topup_status = 'DONE' OR topup_reversed_amount = 0),
    CONSTRAINT refunds_refund_lookup_failures_check CHECK (refund_lookup_failures >= 0),
    CONSTRAINT refunds_refund_called_check CHECK (status NOT IN ('REFUNDING', 'SUCCEEDED') OR refund_called_at IS NOT NULL),
    CONSTRAINT refunds_succeeded_check CHECK (status <> 'SUCCEEDED' OR external_refund_id IS NOT NULL),
    CONSTRAINT refunds_external_state_check CHECK (external_refund_id IS NULL OR status IN ('REFUNDING', 'SUCCEEDED', 'FAILED', 'ANOMALY')),
    CONSTRAINT refunds_external_called_check CHECK (external_refund_id IS NULL OR refund_called_at IS NOT NULL),
    CONSTRAINT refunds_external_id_check CHECK (external_refund_id IS NULL OR char_length(external_refund_id) BETWEEN 1 AND 255),
    CONSTRAINT refunds_failure_check CHECK ((status IN ('FAILED', 'ANOMALY')) = (failure_reason IS NOT NULL)),
    CONSTRAINT refunds_failure_reason_check CHECK (failure_reason IS NULL OR char_length(failure_reason) BETWEEN 1 AND 200)
);
CREATE UNIQUE INDEX refunds_request ON payment.refunds (payment_order_id, request_key);
CREATE UNIQUE INDEX refunds_external ON payment.refunds (external_refund_id) WHERE external_refund_id IS NOT NULL;
CREATE INDEX refunds_order ON payment.refunds (payment_order_id);
CREATE UNIQUE INDEX refunds_topup_external ON payment.refunds (topup_external_id) WHERE topup_external_id IS NOT NULL;
CREATE INDEX refunds_topup_sending ON payment.refunds (updated_at, id) WHERE topup_status = 'SENDING';
CREATE INDEX refunds_open ON payment.refunds (updated_at, id) WHERE status IN ('PENDING', 'REFUNDING');

CREATE TABLE payment.refund_reversals (
    id                 UUID        PRIMARY KEY,
    refund_id          UUID        NOT NULL REFERENCES payment.refunds (id),
    payment_order_id   UUID        NOT NULL REFERENCES payment.payment_orders (id),
    seq                INTEGER     NOT NULL,
    target_refund_id   UUID        REFERENCES payment.refunds (id),
    target_external_id TEXT        NOT NULL,
    amount             BIGINT      NOT NULL,
    status             TEXT        NOT NULL DEFAULT 'REVERSING',
    reason             TEXT,
    idempotency_key    UUID        NOT NULL DEFAULT gen_random_uuid(),
    called_at          TIMESTAMPTZ,
    external_id        TEXT,
    lookup_failures    INTEGER     NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT refund_reversals_seq_check CHECK (seq >= 0),
    CONSTRAINT refund_reversals_target_check CHECK ((seq = 0) = (target_refund_id IS NULL)),
    CONSTRAINT refund_reversals_target_external_check CHECK (char_length(target_external_id) >= 1 AND char_length(target_external_id) <= 255),
    CONSTRAINT refund_reversals_amount_check CHECK (amount > 0),
    CONSTRAINT refund_reversals_status_check CHECK (status IN ('REVERSING', 'REVERSED', 'RELEASED', 'ANOMALY')),
    CONSTRAINT refund_reversals_reason_check CHECK ((status IN ('RELEASED', 'ANOMALY')) = (reason IS NOT NULL) AND (reason IS NULL OR (char_length(reason) >= 1 AND char_length(reason) <= 200))),
    CONSTRAINT refund_reversals_external_check CHECK ((status = 'REVERSED') = (external_id IS NOT NULL) AND (external_id IS NULL OR (char_length(external_id) >= 1 AND char_length(external_id) <= 255))),
    CONSTRAINT refund_reversals_called_check CHECK (external_id IS NULL OR called_at IS NOT NULL),
    CONSTRAINT refund_reversals_lookup_failures_check CHECK (lookup_failures >= 0)
);
CREATE UNIQUE INDEX refund_reversals_seq ON payment.refund_reversals (refund_id, seq);
CREATE UNIQUE INDEX refund_reversals_external ON payment.refund_reversals (external_id) WHERE external_id IS NOT NULL;
CREATE INDEX refund_reversals_reversing ON payment.refund_reversals (updated_at, id) WHERE status = 'REVERSING';
CREATE INDEX refund_reversals_open ON payment.refund_reversals (payment_order_id, target_refund_id) WHERE status IN ('REVERSING', 'ANOMALY');
