
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA coupon;

CREATE TABLE coupon.coupon_campaigns (
    id                  UUID        PRIMARY KEY,
    title               TEXT        NOT NULL,
    discount_type       TEXT        NOT NULL,
    discount_amount     BIGINT,
    discount_percent    INTEGER,
    max_discount_amount BIGINT,
    threshold_amount    BIGINT      NOT NULL DEFAULT 0,
    target_type         TEXT        NOT NULL,
    store_id            BIGINT,
    issuer_type         TEXT        NOT NULL,
    issuer_admin_id     BIGINT,
    issuer_store_id     BIGINT,
    start_at            TIMESTAMPTZ NOT NULL,
    end_at              TIMESTAMPTZ NOT NULL,
    total_limit         INTEGER,
    issued_count        INTEGER     NOT NULL DEFAULT 0,
    status              TEXT        NOT NULL,
    source_type         TEXT,
    source_id           TEXT,
    ended_at            TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT campaigns_title_check CHECK (char_length(title) >= 1 AND char_length(title) <= 100 AND title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT campaigns_discount_type_check CHECK (discount_type IN ('CASH', 'PERCENT')),
    CONSTRAINT campaigns_discount_shape_check CHECK (
        (discount_type = 'CASH' AND discount_amount IS NOT NULL AND discount_amount > 0 AND discount_percent IS NULL AND max_discount_amount IS NULL) OR
        (discount_type = 'PERCENT' AND discount_percent IS NOT NULL AND discount_percent >= 1 AND discount_percent <= 99 AND max_discount_amount IS NOT NULL AND max_discount_amount > 0 AND discount_amount IS NULL)
    ),
    CONSTRAINT campaigns_threshold_check CHECK (threshold_amount >= 0),
    CONSTRAINT campaigns_discount_within_threshold_check CHECK (discount_type <> 'CASH' OR threshold_amount = 0 OR discount_amount <= threshold_amount),
    CONSTRAINT campaigns_target_type_check CHECK (target_type IN ('ALL', 'MEMBER', 'DESIGNATED', 'COMPENSATION')),
    CONSTRAINT campaigns_store_check CHECK (store_id > 0),
    CONSTRAINT campaigns_store_scope_check CHECK (target_type NOT IN ('MEMBER', 'COMPENSATION') OR store_id IS NOT NULL),
    CONSTRAINT campaigns_issuer_check CHECK (
        (issuer_type = 'ADMIN' AND issuer_admin_id IS NOT NULL AND issuer_admin_id > 0 AND issuer_store_id IS NULL) OR
        (issuer_type = 'SELLER' AND issuer_store_id IS NOT NULL AND issuer_store_id > 0 AND issuer_admin_id IS NULL)
    ),
    CONSTRAINT campaigns_seller_scope_check CHECK (issuer_type <> 'SELLER' OR store_id IS NOT DISTINCT FROM issuer_store_id),
    CONSTRAINT campaigns_window_check CHECK (start_at <= end_at),
    CONSTRAINT campaigns_total_limit_check CHECK (total_limit >= 1),
    CONSTRAINT campaigns_issued_count_check CHECK (issued_count >= 0 AND (total_limit IS NULL OR issued_count <= total_limit)),
    CONSTRAINT campaigns_status_check CHECK (status IN ('DRAFT', 'ACTIVE', 'ENDED')),
    CONSTRAINT campaigns_ended_check CHECK ((status = 'ENDED') = (ended_at IS NOT NULL)),
    CONSTRAINT campaigns_source_check CHECK (
        (source_type IS NULL AND source_id IS NULL AND target_type <> 'COMPENSATION') OR
        (source_type IS NOT NULL AND source_id IS NOT NULL AND source_type IN ('AFTERSALES_CASE', 'ADMIN_COMPENSATION') AND char_length(source_id) >= 1 AND char_length(source_id) <= 128 AND target_type = 'COMPENSATION')
    ),
    CONSTRAINT campaigns_source_key UNIQUE (source_type, source_id)
);
CREATE INDEX campaigns_by_issuer_store ON coupon.coupon_campaigns (issuer_store_id, status) WHERE issuer_store_id IS NOT NULL;
CREATE INDEX campaigns_admin_list ON coupon.coupon_campaigns (created_at DESC, id DESC) WHERE target_type <> 'COMPENSATION';

CREATE TABLE coupon.coupon_vouchers (
    id          UUID        PRIMARY KEY,
    campaign_id UUID        NOT NULL REFERENCES coupon.coupon_campaigns (id),
    buyer_id    BIGINT      NOT NULL,
    origin      TEXT        NOT NULL,
    source_type TEXT,
    source_id   TEXT,
    status      TEXT        NOT NULL,
    order_no    TEXT,
    locked_at   TIMESTAMPTZ,
    redeemed_at TIMESTAMPTZ,
    voided_at   TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT vouchers_campaign_buyer_key UNIQUE (campaign_id, buyer_id),
    CONSTRAINT vouchers_source_key UNIQUE (source_type, source_id),
    CONSTRAINT vouchers_buyer_check CHECK (buyer_id > 0),
    CONSTRAINT vouchers_origin_check CHECK (origin IN ('CLAIM', 'GRANT', 'COMPENSATION')),
    CONSTRAINT vouchers_source_check CHECK (
        (origin <> 'COMPENSATION' AND source_type IS NULL AND source_id IS NULL) OR
        (origin = 'COMPENSATION' AND source_type IS NOT NULL AND source_id IS NOT NULL AND source_type IN ('AFTERSALES_CASE', 'ADMIN_COMPENSATION') AND char_length(source_id) >= 1 AND char_length(source_id) <= 128)
    ),
    CONSTRAINT vouchers_status_check CHECK (status IN ('AVAILABLE', 'LOCKED', 'REDEEMED', 'VOIDED')),
    CONSTRAINT vouchers_order_no_check CHECK (char_length(order_no) >= 1 AND char_length(order_no) <= 128),
    CONSTRAINT vouchers_state_check CHECK (
        (status = 'AVAILABLE' AND order_no IS NULL AND locked_at IS NULL AND redeemed_at IS NULL AND voided_at IS NULL) OR
        (status = 'LOCKED' AND order_no IS NOT NULL AND locked_at IS NOT NULL AND redeemed_at IS NULL AND voided_at IS NULL) OR
        (status = 'REDEEMED' AND order_no IS NOT NULL AND locked_at IS NOT NULL AND redeemed_at IS NOT NULL AND voided_at IS NULL) OR
        (status = 'VOIDED' AND order_no IS NULL AND locked_at IS NULL AND redeemed_at IS NULL AND voided_at IS NOT NULL)
    )
);
CREATE UNIQUE INDEX vouchers_order_no_key ON coupon.coupon_vouchers (order_no) WHERE order_no IS NOT NULL;
CREATE INDEX vouchers_by_buyer ON coupon.coupon_vouchers (buyer_id, status, created_at DESC, id DESC);
CREATE INDEX vouchers_by_campaign ON coupon.coupon_vouchers (campaign_id, status);

CREATE TABLE coupon.coupon_target_accounts (
    campaign_id UUID        NOT NULL REFERENCES coupon.coupon_campaigns (id),
    buyer_id    BIGINT      NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (campaign_id, buyer_id),
    CONSTRAINT target_accounts_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX target_accounts_by_buyer ON coupon.coupon_target_accounts (buyer_id);

CREATE TABLE coupon.store_buyer_membership (
    store_id   BIGINT      NOT NULL,
    buyer_id   BIGINT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id),
    CONSTRAINT membership_store_check CHECK (store_id > 0),
    CONSTRAINT membership_buyer_check CHECK (buyer_id > 0)
);
CREATE INDEX membership_by_buyer ON coupon.store_buyer_membership (buyer_id);

CREATE TABLE coupon.seller_coupon_quotas (
    store_id           BIGINT      PRIMARY KEY,
    max_campaign_count INTEGER     NOT NULL DEFAULT 5,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_quotas_store_check CHECK (store_id > 0),
    CONSTRAINT seller_quotas_max_check CHECK (max_campaign_count >= 0)
);

CREATE TABLE coupon.coupon_redemption_logs (
    id                UUID        PRIMARY KEY DEFAULT uuidv7(),
    campaign_id       UUID        NOT NULL,
    voucher_id        UUID,
    buyer_id          BIGINT,
    action            TEXT        NOT NULL,
    order_no          TEXT,
    operator_admin_id BIGINT,
    operator_store_id BIGINT,
    reason            TEXT,
    voided_count      INT,
    created_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT redemption_logs_voided_count_check CHECK (voided_count >= 0),
    CONSTRAINT redemption_logs_action_check CHECK (action IN ('CLAIM', 'ISSUE', 'LOCK', 'REDEEM', 'RELEASE', 'VOID', 'END_CAMPAIGN')),
    CONSTRAINT redemption_logs_reason_check CHECK (char_length(reason) >= 1 AND char_length(reason) <= 255 AND reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')
);
CREATE INDEX redemption_logs_by_voucher ON coupon.coupon_redemption_logs (voucher_id, created_at) WHERE voucher_id IS NOT NULL;
CREATE INDEX redemption_logs_by_campaign ON coupon.coupon_redemption_logs (campaign_id, created_at);

CREATE TABLE coupon.voucher_expiry_reminders (
    voucher_id  UUID        PRIMARY KEY,
    reminded_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX campaigns_by_end_at ON coupon.coupon_campaigns (end_at);
