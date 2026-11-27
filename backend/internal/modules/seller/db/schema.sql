
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA seller;

CREATE TABLE seller.stores (
    store_id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_code           BIGINT NOT NULL,
    account_id           BIGINT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'ACTIVE',
    version              BIGINT NOT NULL DEFAULT 1,
    store_name           TEXT,
    description          TEXT,
    logo_key             TEXT,
    logo_asset_id        UUID,
    cover_key            TEXT,
    cover_asset_id       UUID,
    contact_phone        TEXT,
    contact_name         TEXT,
    contact_mobile       TEXT,
    support_email        TEXT,
    town_code            TEXT,
    address_detail       TEXT,
    prize                TEXT,
    website_url          TEXT,
    onboarding_completed BOOLEAN NOT NULL DEFAULT false,
    created_at           TIMESTAMPTZ NOT NULL,
    updated_at           TIMESTAMPTZ NOT NULL,
    pii_purged_at        TIMESTAMPTZ,
    first_exposed_at     TIMESTAMPTZ,
    current_closed_at    TIMESTAMPTZ,
    retired_at           TIMESTAMPTZ,
    CONSTRAINT stores_status_check CHECK (status IN ('ACTIVE', 'PENDING_DELETION', 'CLOSED')),
    CONSTRAINT stores_closed_images_check CHECK (retired_at IS NULL OR (logo_key IS NULL AND cover_key IS NULL)),
    CONSTRAINT stores_retired_check CHECK (
        retired_at IS NULL OR (status = 'CLOSED' AND description IS NULL AND prize IS NULL AND website_url IS NULL)
    ),
    CONSTRAINT stores_pii_purged_check CHECK (
        (pii_purged_at IS NULL AND store_name IS NOT NULL) OR
        (pii_purged_at IS NOT NULL AND retired_at IS NOT NULL AND status = 'CLOSED' AND
         store_name IS NULL AND contact_phone IS NULL AND contact_name IS NULL AND contact_mobile IS NULL AND
         support_email IS NULL AND town_code IS NULL AND address_detail IS NULL)
    ),
    CONSTRAINT stores_current_closure_check CHECK ((status = 'CLOSED') = (current_closed_at IS NOT NULL)),
    CONSTRAINT stores_version_check CHECK (version >= 1),
    CONSTRAINT stores_store_code_check CHECK (store_code BETWEEN 1 AND 9999999999),
    CONSTRAINT stores_store_name_check CHECK (
        char_length(store_name) >= 1 AND char_length(store_name) <= 100 AND
        store_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'
    ),
    CONSTRAINT stores_description_check CHECK (char_length(description) BETWEEN 1 AND 2000),
    CONSTRAINT stores_images_check CHECK (logo_key <> '' AND cover_key <> ''),
    CONSTRAINT stores_image_assets_check CHECK ((logo_key IS NULL) = (logo_asset_id IS NULL) AND (cover_key IS NULL) = (cover_asset_id IS NULL)),
    CONSTRAINT stores_contact_phone_check CHECK (contact_phone ~ '^0[0-9]{8,9}$'),
    CONSTRAINT stores_contact_name_check CHECK (char_length(contact_name) >= 1 AND char_length(contact_name) <= 50 AND contact_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT stores_contact_mobile_check CHECK (contact_mobile ~ '^09[0-9]{8}$'),
    CONSTRAINT stores_support_email_check CHECK (
        char_length(support_email) <= 254 AND support_email = lower(support_email) AND support_email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'
    ),
    CONSTRAINT stores_town_code_check CHECK (town_code ~ '^[0-9]{8}$'),
    CONSTRAINT stores_address_detail_check CHECK (
        address_detail IS NULL OR (town_code IS NOT NULL AND char_length(address_detail) BETWEEN 1 AND 255)
    ),
    CONSTRAINT stores_prize_check CHECK (char_length(prize) BETWEEN 1 AND 255),
    CONSTRAINT stores_website_url_check CHECK (website_url ~ '^https?://[^/?#]+' AND char_length(website_url) <= 500)
);
ALTER TABLE seller.stores ADD CONSTRAINT stores_store_account_key UNIQUE (store_id, account_id);
ALTER TABLE seller.stores ADD CONSTRAINT stores_current_closure_key UNIQUE (store_id, current_closed_at);
CREATE UNIQUE INDEX stores_store_code ON seller.stores (store_code);
CREATE UNIQUE INDEX stores_current_account ON seller.stores (account_id) WHERE status <> 'CLOSED';
CREATE INDEX stores_account ON seller.stores (account_id);
CREATE INDEX stores_admin_list ON seller.stores (created_at DESC, store_id DESC);
CREATE INDEX stores_retire_pending ON seller.stores (store_id) WHERE status = 'CLOSED' AND retired_at IS NULL;
CREATE INDEX stores_purge_pending ON seller.stores (store_id) WHERE retired_at IS NOT NULL AND pii_purged_at IS NULL;

CREATE TABLE seller.store_closures (
    store_id             BIGINT NOT NULL REFERENCES seller.stores (store_id),
    closed_at            TIMESTAMPTZ NOT NULL,
    closed_by            TEXT NOT NULL,
    closed_by_measure_id UUID,
    reopened_at          TIMESTAMPTZ,
    open_closed_at       TIMESTAMPTZ GENERATED ALWAYS AS (CASE WHEN reopened_at IS NULL THEN closed_at END) STORED,
    PRIMARY KEY (store_id, closed_at),
    CONSTRAINT store_closures_open_key UNIQUE (store_id, open_closed_at),
    CONSTRAINT store_closures_closed_by_check CHECK (closed_by IN ('ACCOUNT_OWNER', 'TERMINATION', 'ACCOUNT_DELETION')),
    CONSTRAINT store_closures_measure_check CHECK ((closed_by = 'TERMINATION') = (closed_by_measure_id IS NOT NULL)),
    CONSTRAINT store_closures_reopened_check CHECK (reopened_at IS NULL OR reopened_at >= closed_at)
);
CREATE UNIQUE INDEX store_closures_current ON seller.store_closures (store_id) WHERE reopened_at IS NULL;
CREATE INDEX store_closures_measure ON seller.store_closures (closed_by_measure_id) WHERE closed_by_measure_id IS NOT NULL;


CREATE TABLE seller.store_exposures (
    store_id             BIGINT PRIMARY KEY REFERENCES seller.stores (store_id),
    exposable            BOOLEAN NOT NULL DEFAULT false,
    version              BIGINT NOT NULL DEFAULT 0,
    exposable_changed_at TIMESTAMPTZ,
    CONSTRAINT store_exposures_version_check CHECK (version >= 0),
    CONSTRAINT store_exposures_exposable_changed_check CHECK ((version = 0) = (exposable_changed_at IS NULL)),
    CONSTRAINT store_exposures_placeholder_check CHECK (version > 0 OR NOT exposable)
);
CREATE INDEX store_exposures_exposable ON seller.store_exposures (store_id) WHERE exposable;

CREATE TABLE seller.cases (
    id                 UUID PRIMARY KEY,
    store_id           BIGINT NOT NULL REFERENCES seller.stores (store_id),
    category           TEXT NOT NULL,
    severity           TEXT NOT NULL,
    version            BIGINT NOT NULL DEFAULT 1,
    opened_at          TIMESTAMPTZ NOT NULL,
    opened_by_admin_id BIGINT NOT NULL,
    closed_at          TIMESTAMPTZ,
    closed_by_admin_id BIGINT,
    close_reason       TEXT,
    decision           TEXT,
    decision_note      TEXT,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT cases_report_key UNIQUE (id, store_id, category),
    CONSTRAINT cases_store_key UNIQUE (id, store_id),
    CONSTRAINT cases_category_check CHECK (category IN (
        'FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT',
        'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT'
    )),
    CONSTRAINT cases_severity_check CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT cases_close_check CHECK ((
        (closed_at IS NULL AND closed_by_admin_id IS NULL AND close_reason IS NULL AND decision IS NULL AND decision_note IS NULL) OR
        (closed_at >= opened_at AND closed_by_admin_id > 0 AND close_reason = 'DECIDED' AND decision IN ('ACTION_TAKEN', 'NO_ACTION') AND
         (decision_note IS NULL OR char_length(decision_note) BETWEEN 1 AND 2000)) OR
        (closed_at >= opened_at AND closed_by_admin_id > 0 AND close_reason = 'NO_REPORTS' AND decision IS NULL AND decision_note IS NULL)
    ) IS TRUE),
    CONSTRAINT cases_decision_note_text_check CHECK (decision_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT cases_admin_check CHECK (opened_by_admin_id > 0),
    CONSTRAINT cases_version_check CHECK (version >= 1),
    CONSTRAINT cases_updated_check CHECK (updated_at >= opened_at)
);
CREATE UNIQUE INDEX cases_one_open ON seller.cases (store_id, category) WHERE closed_at IS NULL;
CREATE INDEX cases_opened ON seller.cases (opened_at DESC, id DESC);
CREATE INDEX cases_open ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NULL;
CREATE INDEX cases_closed ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NOT NULL;
CREATE INDEX cases_store ON seller.cases (store_id, opened_at DESC, id DESC);

CREATE TABLE seller.reports (
    id                    UUID PRIMARY KEY,
    store_id              BIGINT NOT NULL REFERENCES seller.stores (store_id),
    product_id            TEXT,
    order_no              TEXT,
    category              TEXT NOT NULL,
    description           TEXT NOT NULL,
    reporter_buyer_id     BIGINT NOT NULL,
    case_id               UUID,
    assessed_category     TEXT,
    triaged_at            TIMESTAMPTZ,
    triaged_by_admin_id   BIGINT,
    outcome               TEXT,
    resolved_at           TIMESTAMPTZ,
    resolved_by_admin_id  BIGINT,
    resolution_note       TEXT,
    supplements_seen_seq  SMALLINT NOT NULL DEFAULT 0,
    version               BIGINT NOT NULL DEFAULT 1,
    created_at            TIMESTAMPTZ NOT NULL,
    updated_at            TIMESTAMPTZ NOT NULL,
    CONSTRAINT reports_category_check CHECK (category IN (
        'FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT',
        'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'OTHER'
    )),
    CONSTRAINT reports_outcome_check CHECK (outcome IN ('ACTION_TAKEN', 'NO_VIOLATION', 'INSUFFICIENT', 'DUPLICATE', 'ABUSIVE')),
    CONSTRAINT reports_description_check CHECK (char_length(description) BETWEEN 1 AND 2000),
    CONSTRAINT reports_product_id_check CHECK (char_length(product_id) BETWEEN 1 AND 64),
    CONSTRAINT reports_order_no_check CHECK (char_length(order_no) BETWEEN 1 AND 64),
    CONSTRAINT reports_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT reports_triage_check CHECK (
        (case_id IS NULL) = (assessed_category IS NULL) AND (case_id IS NULL) = (triaged_at IS NULL) AND
        (triaged_at IS NULL) = (triaged_by_admin_id IS NULL) AND (triaged_at IS NULL OR triaged_at >= created_at)
    ),
    CONSTRAINT reports_resolved_check CHECK (
        (outcome IS NULL) = (resolved_at IS NULL) AND (resolved_at IS NULL) = (resolved_by_admin_id IS NULL) AND
        (resolution_note IS NULL OR (resolved_at IS NOT NULL AND char_length(resolution_note) BETWEEN 1 AND 2000)) AND
        (resolved_at IS NULL OR resolved_at >= created_at)
    ),
    CONSTRAINT reports_action_taken_check CHECK (outcome IS DISTINCT FROM 'ACTION_TAKEN' OR case_id IS NOT NULL),
    CONSTRAINT reports_admin_check CHECK (triaged_by_admin_id > 0 AND resolved_by_admin_id > 0),
    CONSTRAINT reports_counts_check CHECK (version >= 1),
    CONSTRAINT reports_supplements_seen_check CHECK (supplements_seen_seq BETWEEN 0 AND 10),
    CONSTRAINT reports_store_key UNIQUE (id, store_id),
    CONSTRAINT reports_case_fkey FOREIGN KEY (case_id, store_id, assessed_category) REFERENCES seller.cases (id, store_id, category)
);
CREATE INDEX reports_created ON seller.reports (created_at DESC, id DESC);
CREATE INDEX reports_store ON seller.reports (store_id, created_at DESC, id DESC);
CREATE INDEX reports_reporter ON seller.reports (reporter_buyer_id, created_at DESC, id DESC);
CREATE INDEX reports_awaiting_triage ON seller.reports (created_at, id) WHERE case_id IS NULL AND outcome IS NULL;
CREATE INDEX reports_case ON seller.reports (case_id, created_at, id) WHERE case_id IS NOT NULL;
CREATE UNIQUE INDEX reports_buyer_daily ON seller.reports (
    reporter_buyer_id, store_id, category, ((created_at AT TIME ZONE 'Asia/Taipei')::date)
) WHERE outcome IS NULL;

CREATE TABLE seller.report_supplements (
    id          UUID PRIMARY KEY,
    report_id   UUID NOT NULL REFERENCES seller.reports (id),
    description TEXT NOT NULL,
    seq         SMALLINT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT report_supplements_report_key UNIQUE (id, report_id),
    CONSTRAINT report_supplements_seq_key UNIQUE (id, report_id, seq),
    CONSTRAINT report_supplements_report_seq_key UNIQUE (report_id, seq),
    CONSTRAINT report_supplements_seq_check CHECK (seq BETWEEN 1 AND 10),
    CONSTRAINT report_supplements_description_check CHECK (char_length(description) BETWEEN 1 AND 2000)
);
CREATE INDEX report_supplements_report ON seller.report_supplements (report_id, created_at, id);

CREATE TABLE seller.report_evidence (
    report_id      UUID NOT NULL REFERENCES seller.reports (id),
    submission_seq SMALLINT NOT NULL,
    position       INTEGER NOT NULL,
    supplement_id  UUID,
    asset_id       UUID NOT NULL,
    object_key     TEXT NOT NULL,
    PRIMARY KEY (report_id, submission_seq, position),
    CONSTRAINT report_evidence_submission_check CHECK (submission_seq BETWEEN 0 AND 10),
    CONSTRAINT report_evidence_position_check CHECK (position BETWEEN 0 AND 4),
    CONSTRAINT report_evidence_supplement_check CHECK ((submission_seq = 0) = (supplement_id IS NULL)),
    CONSTRAINT report_evidence_asset_check CHECK (object_key <> ''),
    CONSTRAINT report_evidence_supplement_fkey FOREIGN KEY (supplement_id, report_id, submission_seq) REFERENCES seller.report_supplements (id, report_id, seq)
);

CREATE TABLE seller.report_case_moves (
    id           UUID PRIMARY KEY,
    report_id    UUID NOT NULL,
    store_id     BIGINT NOT NULL,
    from_case_id UUID NOT NULL,
    to_case_id   UUID NOT NULL,
    admin_id     BIGINT NOT NULL,
    moved_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT report_case_moves_report_fkey FOREIGN KEY (report_id, store_id) REFERENCES seller.reports (id, store_id),
    CONSTRAINT report_case_moves_from_fkey FOREIGN KEY (from_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT report_case_moves_to_fkey FOREIGN KEY (to_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT report_case_moves_distinct_check CHECK (from_case_id <> to_case_id),
    CONSTRAINT report_case_moves_admin_check CHECK (admin_id > 0)
);
CREATE INDEX report_case_moves_from ON seller.report_case_moves (from_case_id, moved_at, id);
CREATE INDEX report_case_moves_to ON seller.report_case_moves (to_case_id, moved_at, id);

CREATE TABLE seller.case_inquiries (
    id                         UUID PRIMARY KEY,
    case_id                    UUID NOT NULL,
    store_id                   BIGINT NOT NULL,
    kind                       TEXT NOT NULL,
    message                    TEXT NOT NULL,
    created_at                 TIMESTAMPTZ NOT NULL,
    created_by_admin_id        BIGINT NOT NULL,
    due_at                     TIMESTAMPTZ NOT NULL,
    request_id                 UUID,
    request_hash               BYTEA,
    reply_body                 TEXT,
    reply_channel              TEXT,
    reply_received_at          TIMESTAMPTZ,
    reply_recorded_at          TIMESTAMPTZ,
    reply_recorded_by_admin_id BIGINT,
    replied_by_account_id      BIGINT,
    withdrawn_at               TIMESTAMPTZ,
    withdrawn_by_admin_id      BIGINT,
    withdraw_cause             TEXT,
    CONSTRAINT case_inquiries_case_fkey FOREIGN KEY (case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT case_inquiries_kind_check CHECK (kind IN ('RECORDS', 'EXPLANATION')),
    CONSTRAINT case_inquiries_message_check CHECK (char_length(message) BETWEEN 1 AND 2000),
    CONSTRAINT case_inquiries_due_check CHECK (due_at > created_at),
    CONSTRAINT case_inquiries_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT case_inquiries_request_key UNIQUE (created_by_admin_id, request_id),
    CONSTRAINT case_inquiries_reply_check CHECK ((
        (reply_body IS NULL AND reply_channel IS NULL AND reply_received_at IS NULL AND reply_recorded_at IS NULL
         AND reply_recorded_by_admin_id IS NULL AND replied_by_account_id IS NULL) OR
        (char_length(reply_body) >= 1 AND char_length(reply_body) <= 4000
         AND reply_received_at >= created_at AND reply_recorded_at >= reply_received_at AND (
            (reply_channel IN ('PHONE', 'EMAIL', 'MESSAGING', 'OTHER') AND reply_recorded_by_admin_id > 0 AND replied_by_account_id IS NULL) OR
            (reply_channel = 'SELLER_CENTER' AND replied_by_account_id > 0 AND reply_recorded_by_admin_id IS NULL)
         ))
    ) IS TRUE),
    CONSTRAINT case_inquiries_withdraw_check CHECK ((
        (withdrawn_at IS NULL AND withdrawn_by_admin_id IS NULL AND withdraw_cause IS NULL) OR
        (withdrawn_at >= created_at AND withdrawn_by_admin_id > 0 AND withdraw_cause IN ('ADMIN', 'CASE_CLOSED'))
    ) IS TRUE),
    CONSTRAINT case_inquiries_terminal_check CHECK (reply_received_at IS NULL OR withdrawn_at IS NULL),
    CONSTRAINT case_inquiries_admin_check CHECK (created_by_admin_id > 0)
);
CREATE UNIQUE INDEX case_inquiries_one_open ON seller.case_inquiries (case_id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL;
CREATE INDEX case_inquiries_case ON seller.case_inquiries (case_id, created_at, id);
CREATE INDEX case_inquiries_open_due ON seller.case_inquiries (due_at, id) WHERE reply_received_at IS NULL AND withdrawn_at IS NULL;

CREATE TABLE seller.case_inquiry_reply_evidence (
    inquiry_id UUID NOT NULL REFERENCES seller.case_inquiries (id),
    position   INTEGER NOT NULL,
    asset_id   UUID NOT NULL,
    object_key TEXT NOT NULL,
    PRIMARY KEY (inquiry_id, position),
    CONSTRAINT case_inquiry_reply_evidence_position_check CHECK (position BETWEEN 0 AND 4),
    CONSTRAINT case_inquiry_reply_evidence_asset_check CHECK (object_key <> '')
);

CREATE TABLE seller.violations (
    id                  UUID PRIMARY KEY,
    store_id            BIGINT NOT NULL,
    account_id          BIGINT NOT NULL,
    category            TEXT NOT NULL,
    severity            TEXT NOT NULL,
    escalated_from      TEXT,
    status              TEXT NOT NULL DEFAULT 'ENFORCED',
    origin_measure_id   UUID NOT NULL,
    created_by_admin_id BIGINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    revoked_reason      TEXT,
    revoked_by_admin_id BIGINT,
    revoked_at          TIMESTAMPTZ,
    CONSTRAINT violations_store_account_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    CONSTRAINT violations_category_check CHECK (category IN (
        'FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT',
        'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT'
    )),
    CONSTRAINT violations_severity_check CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT violations_status_check CHECK (status IN ('ENFORCED', 'REVOKED')),
    CONSTRAINT violations_revoked_reason_check CHECK (revoked_reason IN ('APPEAL_ACCEPTED', 'ADMIN_CORRECTION', 'EXTERNAL_DECISION', 'OTHER')),
    CONSTRAINT violations_escalation_check CHECK (escalated_from IS NULL OR (escalated_from = 'MEDIUM' AND severity = 'HIGH')),
    CONSTRAINT violations_revoked_check CHECK (
        (status = 'REVOKED') = (revoked_at IS NOT NULL) AND (revoked_at IS NULL) = (revoked_reason IS NULL) AND
        (revoked_at IS NULL) = (revoked_by_admin_id IS NULL) AND (revoked_at IS NULL OR revoked_at >= created_at)
    ),
    CONSTRAINT violations_admin_check CHECK (created_by_admin_id > 0 AND revoked_by_admin_id > 0)
);
CREATE INDEX violations_created ON seller.violations (created_at DESC, id DESC);
CREATE INDEX violations_store ON seller.violations (store_id, created_at DESC, id DESC);
CREATE INDEX violations_account ON seller.violations (account_id, created_at DESC, id DESC);
CREATE INDEX violations_escalation ON seller.violations (account_id, created_at)
    WHERE status = 'ENFORCED' AND COALESCE(escalated_from, severity) = 'MEDIUM';

CREATE TABLE seller.finding_reports (
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    report_id    UUID NOT NULL REFERENCES seller.reports (id),
    PRIMARY KEY (violation_id, report_id)
);
CREATE INDEX finding_reports_report ON seller.finding_reports (report_id);

CREATE TABLE seller.finding_moderation_reports (
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    report_id    UUID NOT NULL,
    PRIMARY KEY (violation_id, report_id)
);
CREATE INDEX finding_moderation_reports_report ON seller.finding_moderation_reports (report_id);

CREATE TABLE seller.authority_orders (
    id                     UUID PRIMARY KEY,
    agency                 TEXT NOT NULL,
    document_no            TEXT NOT NULL,
    received_at            TIMESTAMPTZ NOT NULL,
    kind                   TEXT NOT NULL,
    summary                TEXT NOT NULL,
    due_at                 TIMESTAMPTZ NOT NULL,
    hold_until             TIMESTAMPTZ NOT NULL,
    registered_at          TIMESTAMPTZ NOT NULL,
    registered_by_admin_id BIGINT NOT NULL,
    completed_at           TIMESTAMPTZ,
    completed_by_admin_id  BIGINT,
    resolution_summary     TEXT,
    version                BIGINT NOT NULL DEFAULT 1,
    request_id             UUID,
    request_hash           BYTEA,
    revoked_at             TIMESTAMPTZ,
    revoked_by_admin_id    BIGINT,
    revoke_reason          TEXT,
    CONSTRAINT authority_orders_kind_check CHECK (kind IN ('RESTRICT_PRODUCT', 'DATA_REQUEST', 'SUSPEND_ACCOUNT', 'OTHER')),
    CONSTRAINT authority_orders_text_check CHECK (
        char_length(agency) BETWEEN 1 AND 100 AND char_length(document_no) BETWEEN 1 AND 100 AND char_length(summary) BETWEEN 1 AND 4000),
    CONSTRAINT authority_orders_times_check CHECK (registered_at >= received_at AND due_at > received_at AND hold_until > received_at),
    CONSTRAINT authority_orders_complete_check CHECK ((
        (completed_at IS NULL AND completed_by_admin_id IS NULL AND resolution_summary IS NULL) OR
        (completed_at >= registered_at AND completed_by_admin_id > 0 AND char_length(resolution_summary) BETWEEN 1 AND 4000)) IS TRUE),
    CONSTRAINT authority_orders_request_check CHECK ((request_id IS NULL) = (request_hash IS NULL)),
    CONSTRAINT authority_orders_request_key UNIQUE (registered_by_admin_id, request_id),
    CONSTRAINT authority_orders_admin_check CHECK (registered_by_admin_id > 0),
    CONSTRAINT authority_orders_version_check CHECK (version >= 1),
    CONSTRAINT authority_orders_revoke_check CHECK ((
        (revoked_at IS NULL AND revoked_by_admin_id IS NULL AND revoke_reason IS NULL) OR
        (revoked_at >= registered_at AND revoked_by_admin_id > 0 AND char_length(revoke_reason) BETWEEN 1 AND 2000)) IS TRUE),
    CONSTRAINT authority_orders_revoke_reason_text_check CHECK (revoke_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT authority_orders_terminal_check CHECK (completed_at IS NULL OR revoked_at IS NULL)
);
CREATE INDEX authority_orders_open_due ON seller.authority_orders (due_at, id) WHERE completed_at IS NULL AND revoked_at IS NULL;
CREATE INDEX authority_orders_received ON seller.authority_orders (received_at DESC, id DESC);

CREATE TABLE seller.authority_order_targets (
    order_id                    UUID NOT NULL REFERENCES seller.authority_orders (id),
    position                    SMALLINT NOT NULL,
    account_id                  BIGINT NOT NULL,
    store_id                    BIGINT,
    product_ids                 TEXT[] NOT NULL DEFAULT '{}',
    notice                      TEXT,
    notified_at                 TIMESTAMPTZ,
    notice_note                 TEXT,
    recheck_on                  DATE,
    notice_recorded_at          TIMESTAMPTZ,
    notice_recorded_by_admin_id BIGINT,
    prev_position               SMALLINT GENERATED ALWAYS AS (CASE WHEN position > 0 THEN position - 1 END) STORED,
    PRIMARY KEY (order_id, position),
    CONSTRAINT authority_order_targets_prev_fkey FOREIGN KEY (order_id, prev_position) REFERENCES seller.authority_order_targets (order_id, position),
    CONSTRAINT authority_order_targets_store_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    CONSTRAINT authority_order_targets_position_check CHECK (position BETWEEN 0 AND 19),
    CONSTRAINT authority_order_targets_products_check CHECK (cardinality(product_ids) <= 50),
    CONSTRAINT authority_order_targets_account_check CHECK (account_id > 0),
    CONSTRAINT authority_order_targets_notice_check CHECK ((
        (notice IS NULL AND notified_at IS NULL AND notice_note IS NULL AND recheck_on IS NULL AND notice_recorded_at IS NULL AND notice_recorded_by_admin_id IS NULL) OR
        (notice = 'NOTIFIED' AND notified_at IS NOT NULL AND recheck_on IS NULL AND notice_recorded_at IS NOT NULL AND notice_recorded_by_admin_id > 0) OR
        (notice = 'WITHHELD' AND notified_at IS NULL AND recheck_on IS NOT NULL AND notice_recorded_at IS NOT NULL AND notice_recorded_by_admin_id > 0) OR
        (notice = 'MEASURE_IS_NOTICE' AND notified_at IS NULL AND recheck_on IS NULL AND notice_recorded_at IS NOT NULL AND notice_recorded_by_admin_id > 0)) IS TRUE),
    CONSTRAINT authority_order_targets_note_check CHECK (notice_note IS NULL OR char_length(notice_note) BETWEEN 1 AND 2000)
);
CREATE UNIQUE INDEX authority_order_targets_unique ON seller.authority_order_targets (order_id, account_id, store_id) NULLS NOT DISTINCT;
CREATE INDEX authority_order_targets_account ON seller.authority_order_targets (account_id, order_id);
CREATE INDEX authority_order_targets_withheld ON seller.authority_order_targets (recheck_on, order_id, position) WHERE notice = 'WITHHELD';

CREATE TABLE seller.authority_order_prepurged_stores (
    order_id UUID NOT NULL REFERENCES seller.authority_orders (id),
    store_id BIGINT NOT NULL REFERENCES seller.stores (store_id),
    PRIMARY KEY (order_id, store_id)
);

CREATE TABLE seller.authority_target_notice_history (
    order_id             UUID NOT NULL,
    position             SMALLINT NOT NULL,
    seq                  SMALLINT NOT NULL,
    notice               TEXT NOT NULL,
    notice_note          TEXT,
    recheck_on           DATE NOT NULL,
    recorded_at          TIMESTAMPTZ NOT NULL,
    recorded_by_admin_id BIGINT NOT NULL,
    replaced_at          TIMESTAMPTZ NOT NULL,
    replaced_by_admin_id BIGINT NOT NULL,
    PRIMARY KEY (order_id, position, seq),
    CONSTRAINT authority_target_notice_history_target_fkey FOREIGN KEY (order_id, position) REFERENCES seller.authority_order_targets (order_id, position),
    CONSTRAINT authority_target_notice_history_notice_check CHECK (notice = 'WITHHELD' AND seq >= 1 AND replaced_at >= recorded_at),
    CONSTRAINT authority_target_notice_history_admin_check CHECK (recorded_by_admin_id > 0 AND replaced_by_admin_id > 0)
);

CREATE TABLE seller.authority_order_hold_extensions (
    order_id   UUID NOT NULL REFERENCES seller.authority_orders (id),
    seq        SMALLINT NOT NULL,
    from_until TIMESTAMPTZ NOT NULL,
    to_until   TIMESTAMPTZ NOT NULL,
    cause      TEXT NOT NULL,
    reason     TEXT,
    admin_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (order_id, seq),
    CONSTRAINT authority_order_hold_extensions_order_check CHECK (to_until > from_until AND seq >= 1),
    CONSTRAINT authority_order_hold_extensions_cause_check CHECK ((
        (cause = 'ADMIN' AND char_length(reason) BETWEEN 1 AND 2000) OR (cause = 'DATA_PROVIDED' AND reason IS NULL)) IS TRUE),
    CONSTRAINT authority_order_hold_extensions_admin_check CHECK (admin_id > 0)
);

CREATE TABLE seller.finding_authority_orders (
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    order_id     UUID NOT NULL REFERENCES seller.authority_orders (id),
    PRIMARY KEY (violation_id, order_id)
);
CREATE INDEX finding_authority_orders_order ON seller.finding_authority_orders (order_id);

CREATE TABLE seller.measures (
    id                  UUID PRIMARY KEY,
    account_id          BIGINT NOT NULL,
    store_id            BIGINT NOT NULL,
    kind                TEXT NOT NULL,
    basis               TEXT NOT NULL,
    risk_ground         TEXT,
    lead_finding_id     UUID,
    review_by           TIMESTAMPTZ,
    statement_to_seller TEXT NOT NULL,
    starts_at           TIMESTAMPTZ NOT NULL,
    ends_at             TIMESTAMPTZ,
    termination_ground  TEXT,
    source_measure_id   UUID REFERENCES seller.measures (id),
    source_case_id      UUID,
    status              TEXT NOT NULL DEFAULT 'ENFORCED',
    version             BIGINT NOT NULL DEFAULT 1,
    current_revision    INTEGER NOT NULL,
    request_id          UUID,
    request_hash        BYTEA,
    created_by_admin_id BIGINT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    revoked_reason      TEXT,
    revoked_by_admin_id BIGINT,
    revoked_at          TIMESTAMPTZ,
    source_authority_order_id UUID REFERENCES seller.authority_orders (id),
    CONSTRAINT measures_store_account_fkey FOREIGN KEY (store_id, account_id) REFERENCES seller.stores (store_id, account_id),
    CONSTRAINT measures_source_case_fkey FOREIGN KEY (source_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT measures_kind_check CHECK (kind IN ('WARNING', 'SUSPENSION', 'TERMINATION', 'CONTENT_TAKEDOWN')),
    CONSTRAINT measures_basis_value_check CHECK (basis IN ('VIOLATION', 'RISK', 'CONTRACT')),
    CONSTRAINT measures_termination_ground_check CHECK (termination_ground IN (
        'NOTICE', 'MATERIAL_BREACH', 'CEASED_BUSINESS', 'DISSOLUTION', 'BANKRUPTCY', 'LICENSE_REVOKED', 'FOOD_SAFETY_INCIDENT', 'FRAUD',
        'PLATFORM_SECURITY', 'OTHER_SERIOUS', 'LINKED_ACCOUNT'
    )),
    CONSTRAINT measures_termination_check CHECK ((kind = 'TERMINATION') = (termination_ground IS NOT NULL)),
    CONSTRAINT measures_contract_check CHECK (basis <> 'CONTRACT' OR kind = 'TERMINATION'),
    CONSTRAINT measures_ground_basis_check CHECK (termination_ground NOT IN ('NOTICE', 'LINKED_ACCOUNT') OR basis = 'CONTRACT'),
    CONSTRAINT measures_breach_check CHECK (termination_ground <> 'MATERIAL_BREACH' OR basis = 'VIOLATION'),
    CONSTRAINT measures_source_check CHECK ((source_measure_id IS NOT NULL) = (termination_ground IS NOT DISTINCT FROM 'LINKED_ACCOUNT')),
    CONSTRAINT measures_starts_check CHECK (
        starts_at >= created_at AND
        (kind = 'TERMINATION' OR starts_at = created_at) AND
        (termination_ground IS DISTINCT FROM 'NOTICE' OR starts_at = created_at + INTERVAL '720 hours') AND
        (termination_ground IS NULL OR termination_ground = 'NOTICE' OR starts_at = created_at)
    ),
    CONSTRAINT measures_ends_check CHECK (ends_at IS NULL OR ends_at > created_at),
    CONSTRAINT measures_risk_ground_check CHECK (risk_ground IN (
        'TRANSACTION', 'FOOD_SAFETY', 'FRAUD', 'PERSONAL_DATA', 'INFRINGEMENT', 'SECURITY', 'FULFILLMENT', 'EXTERNAL_ORDER'
    )),
    CONSTRAINT measures_status_check CHECK (status IN ('ENFORCED', 'REVOKED')),
    CONSTRAINT measures_version_check CHECK (version >= 1),
    CONSTRAINT measures_revision_check CHECK (current_revision >= 1),
    CONSTRAINT measures_basis_check CHECK (
        (basis = 'RISK') = (risk_ground IS NOT NULL) AND (basis = 'VIOLATION') = (lead_finding_id IS NOT NULL) AND
        (basis = 'RISK') = (review_by IS NOT NULL) AND (basis <> 'RISK' OR kind = 'SUSPENSION')
    ),
    CONSTRAINT measures_review_check CHECK (review_by IS NULL OR review_by > created_at),
    CONSTRAINT measures_statement_check CHECK (char_length(statement_to_seller) >= 1 AND char_length(statement_to_seller) <= 2000 AND statement_to_seller !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT measures_takedown_check CHECK (kind <> 'CONTENT_TAKEDOWN' OR (basis = 'VIOLATION' AND starts_at = created_at AND ends_at IS NULL)),
    CONSTRAINT measures_period_check CHECK (
        (kind <> 'WARNING' OR ends_at IS NULL) AND (basis <> 'VIOLATION' OR kind <> 'SUSPENSION' OR ends_at IS NOT NULL)
    ),
    CONSTRAINT measures_request_check CHECK (
        (request_id IS NULL) = (request_hash IS NULL) AND (request_hash IS NULL OR octet_length(request_hash) = 32)
    ),
    CONSTRAINT measures_revoked_reason_check CHECK (revoked_reason IN (
        'APPEAL_ACCEPTED', 'ADMIN_CORRECTION', 'EXTERNAL_DECISION', 'OTHER', 'FINDING_REVOKED', 'SOURCE_REVOKED'
    )),
    CONSTRAINT measures_revoked_check CHECK (
        (status = 'REVOKED') = (revoked_at IS NOT NULL) AND (revoked_at IS NULL) = (revoked_reason IS NULL) AND
        (revoked_at IS NULL) = (revoked_by_admin_id IS NULL) AND (revoked_at IS NULL OR revoked_at >= created_at)
    ),
    CONSTRAINT measures_admin_check CHECK (created_by_admin_id > 0 AND revoked_by_admin_id > 0),
    CONSTRAINT measures_id_basis_key UNIQUE (id, basis)
);
CREATE UNIQUE INDEX measures_request ON seller.measures (created_by_admin_id, request_id) WHERE request_id IS NOT NULL;
CREATE INDEX measures_account ON seller.measures (account_id, created_at DESC, id DESC);
CREATE INDEX measures_created ON seller.measures (created_at DESC, id DESC);
CREATE INDEX measures_enforced_suspension ON seller.measures (account_id, starts_at, ends_at) WHERE status = 'ENFORCED' AND kind IN ('SUSPENSION', 'TERMINATION');
CREATE INDEX measures_active_timed ON seller.measures (ends_at, starts_at) WHERE status = 'ENFORCED' AND kind IN ('SUSPENSION', 'TERMINATION') AND ends_at IS NOT NULL;
CREATE INDEX measures_active_open ON seller.measures (starts_at) WHERE status = 'ENFORCED' AND kind IN ('SUSPENSION', 'TERMINATION') AND ends_at IS NULL;
CREATE INDEX measures_source ON seller.measures (source_measure_id) WHERE source_measure_id IS NOT NULL;
CREATE INDEX measures_source_case ON seller.measures (source_case_id) WHERE source_case_id IS NOT NULL;
CREATE INDEX measures_source_authority_order ON seller.measures (source_authority_order_id) WHERE source_authority_order_id IS NOT NULL;
CREATE UNIQUE INDEX measures_id_status ON seller.measures (id, status);
CREATE UNIQUE INDEX measures_linked_source ON seller.measures (source_measure_id, account_id) WHERE source_measure_id IS NOT NULL AND status = 'ENFORCED';
CREATE INDEX measures_review ON seller.measures (review_by) WHERE status = 'ENFORCED' AND basis = 'RISK';
CREATE INDEX measures_active_takedown ON seller.measures (id) WHERE status = 'ENFORCED' AND kind = 'CONTENT_TAKEDOWN';

CREATE TABLE seller.measure_findings (
    measure_id   UUID NOT NULL REFERENCES seller.measures (id),
    violation_id UUID NOT NULL REFERENCES seller.violations (id),
    PRIMARY KEY (measure_id, violation_id)
);
CREATE INDEX measure_findings_violation ON seller.measure_findings (violation_id);

CREATE TABLE seller.measure_contents (
    measure_id   UUID NOT NULL REFERENCES seller.measures (id),
    content_kind TEXT NOT NULL,
    content_id   UUID NOT NULL,
    measure_status TEXT NOT NULL DEFAULT 'ENFORCED',
    snapshot_state TEXT NOT NULL,
    snapshot_body  TEXT,
    snapshot_authored_at TIMESTAMPTZ,
    snapshot_image_count INTEGER,
    link_post_id   UUID,
    link_store_id  BIGINT,
    link_thread_id UUID,
    PRIMARY KEY (measure_id, content_kind, content_id),
    CONSTRAINT measure_contents_measure_status_fkey FOREIGN KEY (measure_id, measure_status) REFERENCES seller.measures (id, status) ON UPDATE CASCADE,
    CONSTRAINT measure_contents_content_kind_check CHECK (content_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY', 'CHAT_MESSAGE', 'REVIEW_REPLY')),
    CONSTRAINT measure_contents_snapshot_state_check CHECK (snapshot_state IN ('CAPTURED', 'CLEARED')),
    CONSTRAINT measure_contents_snapshot_fields_check CHECK (
        (snapshot_state = 'CAPTURED') = (snapshot_body IS NOT NULL AND snapshot_authored_at IS NOT NULL AND snapshot_image_count IS NOT NULL)
        AND (snapshot_state = 'CAPTURED' OR (snapshot_body IS NULL AND snapshot_authored_at IS NULL AND snapshot_image_count IS NULL
            AND link_post_id IS NULL AND link_store_id IS NULL AND link_thread_id IS NULL))
    ),
    CONSTRAINT measure_contents_snapshot_image_count_check CHECK (snapshot_image_count >= 0),
    CONSTRAINT measure_contents_link_check CHECK (snapshot_state <> 'CAPTURED' OR (
        CASE content_kind
            WHEN 'FORUM_ENTRY' THEN link_post_id IS NULL AND link_store_id IS NOT NULL AND link_store_id > 0 AND link_thread_id IS NOT NULL
            WHEN 'CHAT_MESSAGE' THEN link_post_id IS NULL AND link_store_id IS NULL AND link_thread_id IS NULL
            WHEN 'REVIEW_REPLY' THEN link_post_id IS NULL AND link_store_id IS NULL AND link_thread_id IS NULL
            ELSE link_post_id IS NOT NULL AND link_store_id IS NULL AND link_thread_id IS NULL
        END
    ))
);
CREATE INDEX measure_contents_captured ON seller.measure_contents (measure_id) WHERE snapshot_state = 'CAPTURED' AND measure_status = 'REVOKED';
CREATE UNIQUE INDEX measure_contents_one_enforced ON seller.measure_contents (content_kind, content_id) WHERE measure_status = 'ENFORCED';

CREATE TABLE seller.measure_revisions (
    measure_id          UUID NOT NULL REFERENCES seller.measures (id),
    revision            INTEGER NOT NULL,
    statement_to_seller TEXT NOT NULL,
    ends_at             TIMESTAMPTZ,
    admin_id            BIGINT,
    system_cause        TEXT,
    created_at          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (measure_id, revision),
    CONSTRAINT measure_revisions_revision_check CHECK (revision >= 1),
    CONSTRAINT measure_revisions_statement_check CHECK (char_length(statement_to_seller) >= 1 AND char_length(statement_to_seller) <= 2000 AND statement_to_seller !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT measure_revisions_admin_check CHECK (admin_id > 0),
    CONSTRAINT measure_revisions_system_cause_check CHECK (system_cause IN ('STORE_CLOSED')),
    CONSTRAINT measure_revisions_author_check CHECK ((system_cause IS NULL) = (admin_id IS NOT NULL))
);
ALTER TABLE seller.measures
    ADD CONSTRAINT measures_current_revision_fkey FOREIGN KEY (id, current_revision)
        REFERENCES seller.measure_revisions (measure_id, revision) DEFERRABLE INITIALLY DEFERRED,
    ADD CONSTRAINT measures_lead_finding_fkey FOREIGN KEY (id, lead_finding_id)
        REFERENCES seller.measure_findings (measure_id, violation_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE seller.violations
    ADD CONSTRAINT violations_origin_measure_fkey FOREIGN KEY (origin_measure_id, id)
        REFERENCES seller.measure_findings (measure_id, violation_id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE seller.store_closures
    ADD CONSTRAINT store_closures_measure_fkey FOREIGN KEY (closed_by_measure_id) REFERENCES seller.measures (id),
    ADD CONSTRAINT store_closures_current_fkey FOREIGN KEY (store_id, open_closed_at) REFERENCES seller.stores (store_id, current_closed_at) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE seller.stores
    ADD CONSTRAINT stores_current_closure_fkey FOREIGN KEY (store_id, current_closed_at) REFERENCES seller.store_closures (store_id, open_closed_at) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE seller.governance_notes (
    note_id       UUID PRIMARY KEY,
    measure_id    UUID REFERENCES seller.measures (id),
    case_id       UUID REFERENCES seller.cases (id),
    admin_id      BIGINT NOT NULL,
    body          TEXT NOT NULL,
    severity_from TEXT,
    severity_to   TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT governance_notes_body_check CHECK (char_length(body) BETWEEN 1 AND 2000),
    CONSTRAINT governance_notes_admin_check CHECK (admin_id > 0),
    CONSTRAINT governance_notes_owner_check CHECK (num_nonnulls(case_id, measure_id) = 1),
    CONSTRAINT governance_notes_severity_check CHECK (
        (severity_from IS NULL) = (severity_to IS NULL) AND
        (severity_from IS NULL OR (
            case_id IS NOT NULL AND severity_from <> severity_to AND
            severity_from IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL') AND severity_to IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')
        ))
    )
);
CREATE INDEX governance_notes_measure ON seller.governance_notes (measure_id, created_at, note_id);
CREATE INDEX governance_notes_case ON seller.governance_notes (case_id, created_at, note_id) WHERE case_id IS NOT NULL;

CREATE TABLE seller.measure_requests (
    id                  UUID PRIMARY KEY,
    measure_id          UUID NOT NULL,
    kind                TEXT NOT NULL,
    state               TEXT NOT NULL,
    reason              TEXT NOT NULL,
    revision            INTEGER NOT NULL,
    submitted_at        TIMESTAMPTZ NOT NULL,
    decision_reason     TEXT,
    decided_by_admin_id BIGINT,
    decided_at          TIMESTAMPTZ,
    basis               TEXT NOT NULL,
    close_cause         TEXT,
    CONSTRAINT measure_requests_measure_basis_fkey FOREIGN KEY (measure_id, basis) REFERENCES seller.measures (id, basis),
    CONSTRAINT measure_requests_revision_fkey FOREIGN KEY (measure_id, revision)
        REFERENCES seller.measure_revisions (measure_id, revision),
    CONSTRAINT measure_requests_kind_check CHECK (kind IN ('APPEAL', 'REMEDIATION')),
    CONSTRAINT measure_requests_state_check CHECK (state IN ('PENDING', 'ACCEPTED', 'REJECTED', 'CLOSED')),
    CONSTRAINT measure_requests_close_cause_check CHECK (close_cause IN ('REVOKED', 'ENDED')),
    CONSTRAINT measure_requests_remediation_basis_check CHECK (kind <> 'REMEDIATION' OR basis = 'RISK'),
    CONSTRAINT measure_requests_reason_check CHECK (char_length(reason) >= 1 AND char_length(reason) <= 2000 AND reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT measure_requests_decision_reason_text_check CHECK (decision_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT measure_requests_decision_check CHECK (
        (state = 'PENDING' AND decision_reason IS NULL AND decided_by_admin_id IS NULL AND decided_at IS NULL AND close_cause IS NULL) OR
        (state IN ('ACCEPTED', 'REJECTED') AND decision_reason IS NOT NULL AND char_length(decision_reason) BETWEEN 1 AND 2000 AND
         decided_by_admin_id IS NOT NULL AND decided_by_admin_id > 0 AND decided_at IS NOT NULL AND decided_at >= submitted_at AND
         close_cause IS NULL) OR
        (state = 'CLOSED' AND decision_reason IS NULL AND close_cause IS NOT NULL AND (decided_by_admin_id IS NULL OR decided_by_admin_id > 0) AND
         (close_cause <> 'REVOKED' OR decided_by_admin_id IS NOT NULL) AND decided_at IS NOT NULL AND decided_at >= submitted_at)
    )
);
CREATE UNIQUE INDEX measure_requests_one_appeal ON seller.measure_requests (measure_id) WHERE kind = 'APPEAL';
CREATE UNIQUE INDEX measure_requests_one_pending ON seller.measure_requests (measure_id) WHERE state = 'PENDING';
CREATE INDEX measure_requests_pending ON seller.measure_requests (submitted_at, id) WHERE state = 'PENDING';

CREATE TABLE seller.measure_request_evidence (
    request_id UUID NOT NULL REFERENCES seller.measure_requests (id),
    position   INTEGER NOT NULL,
    asset_id   UUID NOT NULL,
    object_key TEXT NOT NULL,
    PRIMARY KEY (request_id, position),
    CONSTRAINT measure_request_evidence_position_check CHECK (position >= 0 AND position < 5),
    CONSTRAINT measure_request_evidence_asset_check CHECK (object_key <> '')
);

CREATE TABLE seller.evidence_accesses (
    access_id      UUID PRIMARY KEY,
    report_id      UUID,
    submission_seq SMALLINT,
    request_id     UUID,
    inquiry_id     UUID,
    position       INTEGER NOT NULL,
    admin_id       BIGINT NOT NULL,
    accessed_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT evidence_accesses_report_fkey FOREIGN KEY (report_id, submission_seq, position) REFERENCES seller.report_evidence (report_id, submission_seq, position),
    CONSTRAINT evidence_accesses_seq_check CHECK ((report_id IS NULL) = (submission_seq IS NULL)),
    CONSTRAINT evidence_accesses_request_fkey FOREIGN KEY (request_id, position) REFERENCES seller.measure_request_evidence (request_id, position),
    CONSTRAINT evidence_accesses_inquiry_fkey FOREIGN KEY (inquiry_id, position) REFERENCES seller.case_inquiry_reply_evidence (inquiry_id, position),
    CONSTRAINT evidence_accesses_owner_check CHECK (num_nonnulls(report_id, request_id, inquiry_id) = 1),
    CONSTRAINT evidence_accesses_admin_check CHECK (admin_id > 0)
);
CREATE INDEX evidence_accesses_report ON seller.evidence_accesses (report_id, accessed_at DESC) WHERE report_id IS NOT NULL;
CREATE INDEX evidence_accesses_request ON seller.evidence_accesses (request_id, accessed_at DESC) WHERE request_id IS NOT NULL;
CREATE INDEX evidence_accesses_inquiry ON seller.evidence_accesses (inquiry_id, accessed_at DESC) WHERE inquiry_id IS NOT NULL;

CREATE TABLE seller.suspension_periods (
    period_id  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id   BIGINT NOT NULL REFERENCES seller.stores (store_id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at   TIMESTAMPTZ,
    CONSTRAINT suspension_periods_ended_check CHECK (ended_at IS NULL OR ended_at >= started_at)
);
CREATE UNIQUE INDEX suspension_periods_open ON seller.suspension_periods (store_id) WHERE ended_at IS NULL;

CREATE TABLE seller.measure_identities (
    measure_id UUID NOT NULL REFERENCES seller.measures (id),
    kind       TEXT NOT NULL,
    key_id     TEXT NOT NULL,
    hash       BYTEA NOT NULL,
    PRIMARY KEY (measure_id, kind, hash),
    CONSTRAINT measure_identities_kind_check CHECK (kind IN ('TAX_ID', 'BANK_ACCOUNT')),
    CONSTRAINT measure_identities_key_check CHECK (key_id <> ''),
    CONSTRAINT measure_identities_hash_check CHECK (octet_length(hash) = 32)
);
CREATE INDEX measure_identities_hash ON seller.measure_identities (hash);

CREATE TABLE seller.measure_linked_reviews (
    measure_id           UUID NOT NULL REFERENCES seller.measures (id),
    account_id           BIGINT NOT NULL,
    reviewed_at          TIMESTAMPTZ NOT NULL,
    reviewed_by_admin_id BIGINT NOT NULL,
    PRIMARY KEY (measure_id, account_id),
    CONSTRAINT measure_linked_reviews_admin_check CHECK (reviewed_by_admin_id > 0)
);

CREATE TABLE seller.kyc_applications (
    application_id          UUID PRIMARY KEY,
    store_id                BIGINT NOT NULL REFERENCES seller.stores (store_id),
    status                  TEXT NOT NULL DEFAULT 'DRAFT',
    version                 BIGINT NOT NULL DEFAULT 1,
    seller_type             TEXT NOT NULL,
    product_type            TEXT,
    website_url             TEXT,
    company_name            TEXT,
    tax_id                  TEXT,
    company_register_date   DATE,
    company_town_code       TEXT,
    company_address_detail  TEXT,
    company_phone           TEXT,
    submitted_at            TIMESTAMPTZ,
    submission_seq          BIGINT NOT NULL DEFAULT 0,
    decided_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    pii_purged_at           TIMESTAMPTZ,
    CONSTRAINT kyc_applications_status_check CHECK (status IN ('DRAFT', 'SUBMITTED', 'APPROVED', 'REJECTED', 'CANCELLED')),
    CONSTRAINT kyc_applications_seller_type_check CHECK (seller_type IN ('INDIVIDUAL', 'COMPANY')),
    CONSTRAINT kyc_applications_version_check CHECK (version >= 1),
    CONSTRAINT kyc_applications_submission_seq_check CHECK (submission_seq >= 0 AND (status IN ('DRAFT', 'CANCELLED') OR submission_seq >= 1)),
    CONSTRAINT kyc_applications_product_type_check CHECK (char_length(product_type) BETWEEN 1 AND 100),
    CONSTRAINT kyc_applications_website_check CHECK (website_url ~ '^https?://[^/?#]+' AND char_length(website_url) <= 500),
    CONSTRAINT kyc_applications_company_name_check CHECK (char_length(company_name) BETWEEN 1 AND 200),
    CONSTRAINT kyc_applications_tax_id_check CHECK (tax_id ~ '^[0-9]{8}$'),
    CONSTRAINT kyc_applications_company_register_date_check CHECK (company_register_date >= DATE '1900-01-01'),
    CONSTRAINT kyc_applications_company_town_check CHECK (company_town_code ~ '^[0-9]{8}$'),
    CONSTRAINT kyc_applications_company_address_check CHECK (
        company_address_detail IS NULL OR (company_town_code IS NOT NULL AND char_length(company_address_detail) BETWEEN 1 AND 255)
    ),
    CONSTRAINT kyc_applications_company_phone_check CHECK (company_phone ~ '^0[0-9]{8,9}$'),
    CONSTRAINT kyc_applications_type_fields_check CHECK (
        seller_type = 'COMPANY' OR
        (company_name IS NULL AND tax_id IS NULL AND company_register_date IS NULL AND company_town_code IS NULL AND
         company_address_detail IS NULL AND company_phone IS NULL)
    ),
    CONSTRAINT kyc_applications_complete_check CHECK (
        status NOT IN ('SUBMITTED', 'APPROVED', 'REJECTED') OR (
            submitted_at IS NOT NULL AND (pii_purged_at IS NOT NULL OR (
                (seller_type = 'INDIVIDUAL' OR
                 (seller_type = 'COMPANY' AND company_name IS NOT NULL AND tax_id IS NOT NULL))
            ))
        )
    ),
    CONSTRAINT kyc_applications_purged_check CHECK (
        pii_purged_at IS NULL OR (
            website_url IS NULL AND company_name IS NULL AND tax_id IS NULL AND
            company_register_date IS NULL AND company_town_code IS NULL AND company_address_detail IS NULL AND company_phone IS NULL
        )
    ),
    CONSTRAINT kyc_applications_decided_check CHECK ((status IN ('APPROVED', 'REJECTED')) = (decided_at IS NOT NULL))
);
CREATE UNIQUE INDEX kyc_applications_open ON seller.kyc_applications (store_id) WHERE status <> 'CANCELLED';
CREATE INDEX kyc_applications_store ON seller.kyc_applications (store_id, created_at DESC, application_id DESC);
CREATE INDEX kyc_applications_status_submitted ON seller.kyc_applications (status, submitted_at, application_id)
    WHERE status <> 'DRAFT' AND submitted_at IS NOT NULL;
CREATE INDEX kyc_applications_submitted ON seller.kyc_applications (submitted_at, application_id)
    WHERE status <> 'DRAFT' AND submitted_at IS NOT NULL;

CREATE TABLE seller.account_identity_hashes (
    source_store_id BIGINT NOT NULL REFERENCES seller.stores (store_id),
    field           TEXT NOT NULL,
    key_id          TEXT NOT NULL,
    hash            BYTEA NOT NULL,
    account_id      BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (source_store_id, field, key_id, hash),
    CONSTRAINT account_identity_hashes_field_check CHECK (field IN ('TAX_ID', 'BANK_FINGERPRINT')),
    CONSTRAINT account_identity_hashes_key_check CHECK (key_id <> ''),
    CONSTRAINT account_identity_hashes_hash_check CHECK (octet_length(hash) = 32)
);
CREATE INDEX account_identity_hashes_account ON seller.account_identity_hashes (account_id);
CREATE INDEX account_identity_hashes_hash ON seller.account_identity_hashes (hash, key_id);

CREATE TABLE seller.kyc_documents (
    application_id UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    position       INTEGER NOT NULL DEFAULT 0,
    asset_id       UUID NOT NULL,
    object_key     TEXT NOT NULL,
    PRIMARY KEY (application_id, position),
    CONSTRAINT kyc_documents_position_check CHECK (position >= 0 AND position < 10),
    CONSTRAINT kyc_documents_asset_check CHECK (object_key <> '')
);

CREATE TABLE seller.kyc_registration_checks (
    check_id               UUID PRIMARY KEY,
    application_id         UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    seq                    BIGINT NOT NULL,
    request_kind           TEXT NOT NULL,
    requested_by_admin_id  BIGINT,
    requested_at           TIMESTAMPTZ NOT NULL,
    deadline_at            TIMESTAMPTZ NOT NULL,
    submission_seq         BIGINT NOT NULL,
    outcome                TEXT NOT NULL DEFAULT 'PENDING',
    queried_at             TIMESTAMPTZ,
    tax_id                 TEXT,
    name                   TEXT,
    address_text           TEXT,
    town_code              TEXT,
    address_detail         TEXT,
    setup_date             DATE,
    organization_type      TEXT,
    status                 TEXT,
    status_text            TEXT,
    suspended_from         DATE,
    suspended_until        DATE,
    mismatched_fields      TEXT[] NOT NULL DEFAULT '{}',
    unverifiable_fields    TEXT[] NOT NULL DEFAULT '{}',
    CONSTRAINT kyc_registration_checks_seq_key UNIQUE (application_id, seq),
    CONSTRAINT kyc_registration_checks_seq_check CHECK (seq >= 1),
    CONSTRAINT kyc_registration_checks_deadline_check CHECK (deadline_at > requested_at),
    CONSTRAINT kyc_registration_checks_submission_seq_check CHECK (submission_seq >= 1),
    CONSTRAINT kyc_registration_checks_kind_check CHECK (
        (request_kind = 'SUBMIT' AND requested_by_admin_id IS NULL) OR
        (request_kind = 'ADMIN_REQUERY' AND requested_by_admin_id IS NOT NULL AND requested_by_admin_id > 0)
    ),
    CONSTRAINT kyc_registration_checks_outcome_check CHECK (outcome IN ('PENDING', 'FOUND', 'NOT_FOUND', 'UNAVAILABLE', 'CANCELLED')),
    CONSTRAINT kyc_registration_checks_queried_check CHECK ((outcome = 'PENDING') = (queried_at IS NULL)),
    CONSTRAINT kyc_registration_checks_found_check CHECK (
        (outcome = 'FOUND' AND tax_id IS NOT NULL AND tax_id ~ '^[0-9]{8}$' AND name IS NOT NULL AND char_length(name) >= 1
            AND address_text IS NOT NULL AND status IS NOT NULL AND status IN ('ACTIVE', 'ORDERED_TO_DISSOLVE', 'TERMINATED', 'UNKNOWN') AND status_text IS NOT NULL) OR
        (outcome <> 'FOUND' AND tax_id IS NULL AND name IS NULL AND address_text IS NULL AND town_code IS NULL
            AND address_detail IS NULL AND setup_date IS NULL AND organization_type IS NULL AND status IS NULL
            AND status_text IS NULL AND suspended_from IS NULL AND suspended_until IS NULL
            AND mismatched_fields = '{}' AND unverifiable_fields = '{}')
    ),
    CONSTRAINT kyc_registration_checks_town_check CHECK (
        (town_code IS NULL) = (address_detail IS NULL) AND (town_code IS NULL OR town_code ~ '^[0-9]{8}$')
    ),
    CONSTRAINT kyc_registration_checks_suspension_check CHECK (suspended_from IS NULL OR suspended_until IS NULL OR suspended_until >= suspended_from),
    CONSTRAINT kyc_registration_checks_fields_check CHECK (
        mismatched_fields <@ ARRAY['business.company_name', 'business.company_address', 'business.company_register_date']::text[] AND
        unverifiable_fields <@ ARRAY['business.company_name', 'business.company_address', 'business.company_register_date']::text[] AND
        NOT (mismatched_fields && unverifiable_fields)
    )
);
CREATE INDEX kyc_registration_checks_pending ON seller.kyc_registration_checks (deadline_at) WHERE outcome = 'PENDING';

CREATE TABLE seller.kyc_reviews (
    review_id          UUID PRIMARY KEY,
    application_id     UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    action             TEXT NOT NULL,
    reviewer_admin_id  BIGINT,
    reason             TEXT,
    internal_note      TEXT,
    decided_at         TIMESTAMPTZ NOT NULL,
    registration_check_id UUID REFERENCES seller.kyc_registration_checks (check_id) ON DELETE SET NULL,
    CONSTRAINT kyc_reviews_action_check CHECK (action IN ('SUBMIT', 'APPROVE', 'REJECT', 'CANCEL')),
    CONSTRAINT kyc_reviews_reviewer_check CHECK (
        (action IN ('APPROVE', 'REJECT') AND reviewer_admin_id IS NOT NULL AND reviewer_admin_id > 0) OR
        (action IN ('SUBMIT', 'CANCEL') AND reviewer_admin_id IS NULL)
    ),
    CONSTRAINT kyc_reviews_reason_check CHECK (char_length(reason) BETWEEN 1 AND 1000),
    CONSTRAINT kyc_reviews_reject_reason_check CHECK (action <> 'REJECT' OR reason IS NOT NULL),
    CONSTRAINT kyc_reviews_internal_note_check CHECK (internal_note IS NULL OR (action = 'REJECT' AND char_length(internal_note) BETWEEN 1 AND 1000)),
    CONSTRAINT kyc_reviews_registration_check_check CHECK (registration_check_id IS NULL OR action IN ('APPROVE', 'REJECT'))
);
CREATE INDEX kyc_reviews_application ON seller.kyc_reviews (application_id, decided_at, review_id);
CREATE INDEX kyc_reviews_registration_check ON seller.kyc_reviews (registration_check_id) WHERE registration_check_id IS NOT NULL;

CREATE TABLE seller.kyc_review_issues (
    issue_id           UUID PRIMARY KEY,
    review_id          UUID NOT NULL REFERENCES seller.kyc_reviews (review_id),
    position           INTEGER NOT NULL,
    field              TEXT NOT NULL,
    document_asset_id  UUID,
    code               TEXT NOT NULL,
    message            TEXT NOT NULL,
    resolved_at        TIMESTAMPTZ,
    CONSTRAINT kyc_review_issues_position_key UNIQUE (review_id, position),
    CONSTRAINT kyc_review_issues_position_check CHECK (position >= 0),
    CONSTRAINT kyc_review_issues_field_check CHECK (field IN (
        'product_type',
        'website_url',
        'business.company_name',
        'business.tax_id',
        'business.company_register_date',
        'business.company_address',
        'business.company_phone',
        'business.company_documents'
    )),
    CONSTRAINT kyc_review_issues_code_check CHECK (code IN ('UNREADABLE', 'MISMATCH', 'EXPIRED', 'MISSING', 'INVALID', 'OTHER')),
    CONSTRAINT kyc_review_issues_message_check CHECK (char_length(message) BETWEEN 1 AND 500),
    CONSTRAINT kyc_review_issues_asset_check CHECK (
        document_asset_id IS NULL OR
        field = 'business.company_documents'
    )
);

CREATE TABLE seller.kyc_document_accesses (
    access_id      UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES seller.kyc_applications (application_id),
    position       INTEGER NOT NULL,
    asset_id       UUID NOT NULL,
    admin_id       BIGINT NOT NULL,
    accessed_at    TIMESTAMPTZ NOT NULL,
        CONSTRAINT kyc_document_accesses_position_check CHECK (position >= 0 AND position < 10),
    CONSTRAINT kyc_document_accesses_admin_check CHECK (admin_id > 0)
);
CREATE INDEX kyc_document_accesses_application ON seller.kyc_document_accesses (application_id, accessed_at DESC, access_id DESC);

CREATE TABLE seller.prospects (
    prospect_id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version                 BIGINT NOT NULL DEFAULT 1,
    display_name            TEXT NOT NULL,
    vendor_type             TEXT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'ACTIVE',
    outreach_status         TEXT NOT NULL DEFAULT 'NOT_CONTACTED',
    live                    BOOLEAN GENERATED ALWAYS AS (status <> 'ARCHIVED') STORED,
    tax_id                  TEXT,
    converted_store_id      BIGINT REFERENCES seller.stores (store_id),
    converted_at            TIMESTAMPTZ,
    merged_into             BIGINT REFERENCES seller.prospects (prospect_id),
    category                TEXT,
    region                  TEXT,
    product_summary         TEXT,
    positioning             TEXT,
    research_summary        TEXT,
    priority_letter         TEXT,
    priority_stars          INTEGER NOT NULL DEFAULT 0,
    logo_key                TEXT,
    logo_asset_id           UUID,
    logo_source_url         TEXT,
    cover_key               TEXT,
    cover_asset_id          UUID,
    cover_source_url        TEXT,
    created_by_admin_id     BIGINT,
    updated_by_admin_id     BIGINT,
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    CONSTRAINT prospects_version_check CHECK (version >= 1),
    CONSTRAINT prospects_display_name_check CHECK (char_length(display_name) >= 1 AND char_length(display_name) <= 255 AND display_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'),
    CONSTRAINT prospects_vendor_type_check CHECK (vendor_type IN ('INDIVIDUAL', 'BUSINESS')),
    CONSTRAINT prospects_status_check CHECK (status IN ('ACTIVE', 'ARCHIVED', 'CONVERTED')),
    CONSTRAINT prospects_outreach_check CHECK (outreach_status IN ('NOT_CONTACTED', 'INVITED', 'REPLIED', 'DECLINED', 'DO_NOT_CONTACT', 'CONVERTED')),
    CONSTRAINT prospects_tax_id_check CHECK (tax_id ~ '^[0-9]{8}$'),
    CONSTRAINT prospects_converted_check CHECK (
        (status = 'CONVERTED') = (converted_store_id IS NOT NULL) AND (converted_store_id IS NULL) = (converted_at IS NULL) AND
        (status = 'CONVERTED') = (outreach_status = 'CONVERTED')
    ),
    CONSTRAINT prospects_merged_check CHECK (merged_into IS NULL OR (status = 'ARCHIVED' AND merged_into <> prospect_id)),
    CONSTRAINT prospects_priority_check CHECK (priority_stars >= 0 AND char_length(priority_letter) >= 1 AND char_length(priority_letter) <= 4),
    CONSTRAINT prospects_text_check CHECK (
        char_length(category) >= 1 AND char_length(category) <= 100 AND
        char_length(region) >= 1 AND char_length(region) <= 100 AND
        char_length(product_summary) >= 1 AND char_length(positioning) >= 1 AND char_length(research_summary) >= 1
    ),
    CONSTRAINT prospects_images_check CHECK (
        ((logo_key IS NULL) = (logo_asset_id IS NULL)) AND ((cover_key IS NULL) = (cover_asset_id IS NULL)) AND
        (status <> 'ARCHIVED' OR (logo_key IS NULL AND cover_key IS NULL))
    ),
    CONSTRAINT prospects_source_urls_check CHECK ((logo_key IS NOT NULL OR logo_source_url IS NULL) AND (cover_key IS NOT NULL OR cover_source_url IS NULL)),
    CONSTRAINT prospects_admin_check CHECK (created_by_admin_id > 0 AND updated_by_admin_id > 0)
);
CREATE UNIQUE INDEX prospects_tax_id ON seller.prospects (tax_id) WHERE tax_id IS NOT NULL AND live;
CREATE UNIQUE INDEX prospects_converted_store ON seller.prospects (converted_store_id) WHERE converted_store_id IS NOT NULL;
CREATE INDEX prospects_list ON seller.prospects (updated_at DESC, prospect_id DESC);
CREATE INDEX prospects_active_name ON seller.prospects (lower(display_name), prospect_id) WHERE status = 'ACTIVE';
CREATE INDEX prospects_merged_into ON seller.prospects (merged_into) WHERE merged_into IS NOT NULL;

CREATE TABLE seller.prospect_contacts (
    prospect_id    BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    kind           TEXT NOT NULL,
    value          TEXT NOT NULL,
    match_key      TEXT,
    is_primary     BOOLEAN NOT NULL,
    primary_marker BOOLEAN GENERATED ALWAYS AS (CASE WHEN is_primary THEN true END) STORED,
    has_contact    BOOLEAN GENERATED ALWAYS AS (true) STORED,
    PRIMARY KEY (prospect_id, kind, value),
    CONSTRAINT prospect_contacts_primary UNIQUE (prospect_id, kind, primary_marker),
    CONSTRAINT prospect_contacts_has_primary FOREIGN KEY (prospect_id, kind, has_contact)
        REFERENCES seller.prospect_contacts (prospect_id, kind, primary_marker) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT prospect_contacts_kind_check CHECK (kind IN ('EMAIL', 'PHONE', 'WEBSITE', 'SOCIAL')),
    CONSTRAINT prospect_contacts_value_check CHECK (
        char_length(value) >= 1 AND value !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$' AND
        char_length(value) <= CASE kind WHEN 'EMAIL' THEN 255 WHEN 'PHONE' THEN 50 ELSE 500 END
    ),
    CONSTRAINT prospect_contacts_match_key_check CHECK (
        match_key IS NULL OR (char_length(match_key) <= 600 AND CASE kind
            WHEN 'EMAIL' THEN match_key ~ '^EMAIL:.+$'
            WHEN 'PHONE' THEN match_key ~ '^PHONE:.+$'
            WHEN 'WEBSITE' THEN match_key ~ '^(DOMAIN|SOCIAL):.+$'
            ELSE match_key ~ '^SOCIAL:.+$'
        END)
    )
);
CREATE INDEX prospect_contacts_match_key ON seller.prospect_contacts (match_key, prospect_id) WHERE match_key IS NOT NULL;

CREATE TABLE seller.prospect_distinct_pairs (
    low_prospect_id       BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    high_prospect_id      BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    confirmed_by_admin_id BIGINT NOT NULL,
    confirmed_at          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (low_prospect_id, high_prospect_id),
    CONSTRAINT prospect_distinct_pairs_order_check CHECK (low_prospect_id < high_prospect_id),
    CONSTRAINT prospect_distinct_pairs_admin_check CHECK (confirmed_by_admin_id > 0)
);
CREATE INDEX prospect_distinct_pairs_high ON seller.prospect_distinct_pairs (high_prospect_id);

CREATE TABLE seller.prospect_categories (
    prospect_id BIGINT NOT NULL REFERENCES seller.prospects (prospect_id),
    category_id TEXT NOT NULL,
    PRIMARY KEY (prospect_id, category_id),
    CONSTRAINT prospect_categories_category_check CHECK (char_length(category_id) BETWEEN 1 AND 64)
);
CREATE INDEX prospect_categories_category ON seller.prospect_categories (category_id);

CREATE TABLE seller.product_view_events (
    event_id     UUID PRIMARY KEY,
    product_id   UUID NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    store_id     BIGINT,
    processed_at TIMESTAMPTZ,
    outcome      TEXT,
    CONSTRAINT product_view_events_product_check CHECK (product_id <> '00000000-0000-0000-0000-000000000000'::uuid),
    CONSTRAINT product_view_events_outcome_check CHECK (outcome IN ('COUNTED', 'UNRESOLVED')),
    CONSTRAINT product_view_events_processed_check CHECK (
        (processed_at IS NULL AND outcome IS NULL AND store_id IS NULL)
        OR (processed_at IS NOT NULL AND outcome = 'COUNTED' AND store_id IS NOT NULL AND store_id > 0)
        OR (processed_at IS NOT NULL AND outcome = 'UNRESOLVED' AND store_id IS NULL)
    )
);
CREATE INDEX product_view_events_pending ON seller.product_view_events (occurred_at, event_id) WHERE processed_at IS NULL;
CREATE INDEX product_view_events_processed ON seller.product_view_events (occurred_at) WHERE processed_at IS NOT NULL;

CREATE TABLE seller.product_views_daily (
    store_id BIGINT NOT NULL REFERENCES seller.stores (store_id),
    day      DATE NOT NULL,
    views    BIGINT NOT NULL,
    PRIMARY KEY (store_id, day),
    CONSTRAINT product_views_daily_views_check CHECK (views >= 1)
);

CREATE TABLE seller.agreement_acceptances (
    acceptance_id        BIGINT      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id             BIGINT      NOT NULL REFERENCES seller.stores (store_id),
    kind                 TEXT        NOT NULL,
    version              TEXT        NOT NULL,
    account_id           BIGINT      NOT NULL,
    accepted_at          TIMESTAMPTZ NOT NULL,
    submission_review_id UUID        REFERENCES seller.kyc_reviews (review_id),
    client_ip            INET,
    user_agent           TEXT,
    pii_purged_at        TIMESTAMPTZ,
    CONSTRAINT agreement_acceptances_kind_check CHECK (kind IN ('SELLER_TERMS', 'KYC_PRIVACY_NOTICE')),
    CONSTRAINT agreement_acceptances_version_check CHECK (char_length(version) >= 1),
    CONSTRAINT agreement_acceptances_account_check CHECK (account_id > 0),
    CONSTRAINT agreement_acceptances_user_agent_check CHECK (user_agent IS NULL OR char_length(user_agent) BETWEEN 1 AND 512),
    CONSTRAINT agreement_acceptances_source_check CHECK (
        (kind = 'SELLER_TERMS' AND submission_review_id IS NULL) OR
        (kind = 'KYC_PRIVACY_NOTICE' AND submission_review_id IS NOT NULL)
    ),
    CONSTRAINT agreement_acceptances_purged_check CHECK (pii_purged_at IS NULL OR (client_ip IS NULL AND user_agent IS NULL))
);
CREATE UNIQUE INDEX agreement_acceptances_seller_terms ON seller.agreement_acceptances (store_id) WHERE kind = 'SELLER_TERMS';
CREATE INDEX agreement_acceptances_submission ON seller.agreement_acceptances (submission_review_id) WHERE submission_review_id IS NOT NULL;
CREATE INDEX agreement_acceptances_store ON seller.agreement_acceptances (store_id, accepted_at, acceptance_id);

CREATE TABLE seller.store_blocks (
    store_id   BIGINT NOT NULL REFERENCES seller.stores (store_id),
    buyer_id   BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (store_id, buyer_id),
    CONSTRAINT store_blocks_buyer_id_check CHECK (buyer_id > 0)
);
CREATE INDEX store_blocks_buyer ON seller.store_blocks (buyer_id, store_id);

CREATE TABLE seller.payout_capabilities (
    store_id         BIGINT      PRIMARY KEY REFERENCES seller.stores (store_id),
    provider         TEXT        NOT NULL,
    transfers_active BOOLEAN     NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL,
    CONSTRAINT payout_capabilities_provider_check CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$')
);
