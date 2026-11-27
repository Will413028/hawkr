
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA moderation;

CREATE TABLE moderation.cases (
    id                  UUID        PRIMARY KEY,
    target_kind         TEXT        NOT NULL,
    content_id          UUID,
    buyer_id            BIGINT,
    version             BIGINT      NOT NULL DEFAULT 1,
    opened_at           TIMESTAMPTZ NOT NULL,
    decision            TEXT,
    decision_category   TEXT,
    decision_action     TEXT,
    decided_by_admin_id BIGINT,
    closed_at           TIMESTAMPTZ,
    measure_id          UUID,
    reversed_at          TIMESTAMPTZ,
    reversed_by_admin_id BIGINT,
    reverse_reason       TEXT,
    CONSTRAINT cases_target_kind_check CHECK (target_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY', 'BUYER', 'REVIEW', 'CHAT_MESSAGE', 'REVIEW_REPLY')),
    CONSTRAINT cases_target_check CHECK (
        (target_kind = 'BUYER' AND buyer_id > 0 AND content_id IS NULL)
        OR (target_kind <> 'BUYER' AND content_id IS NOT NULL AND buyer_id IS NULL)
    ),
    CONSTRAINT cases_version_check CHECK (version >= 1),
    CONSTRAINT cases_decision_check CHECK (decision IN ('VIOLATION', 'NO_VIOLATION')),
    CONSTRAINT cases_decision_category_check CHECK (decision_category IN ('FOOD_SAFETY', 'PROHIBITED_GOODS', 'UNLICENSED_SALE', 'MISLEADING_INFO', 'FAKE_CERTIFICATION', 'COUNTERFEIT', 'IP_INFRINGEMENT', 'FRAUD', 'OFF_PLATFORM_TRADE', 'PRIVACY', 'IMPERSONATION', 'PLATFORM_ABUSE', 'SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT')),
    CONSTRAINT cases_decision_action_check CHECK (decision_action IN ('HIDE', 'WARN', 'BAN', 'TAKEDOWN')),
    CONSTRAINT cases_decided_by_check CHECK (decided_by_admin_id > 0),
    CONSTRAINT cases_closed_together_check CHECK (
        (decision IS NULL AND decided_by_admin_id IS NULL AND closed_at IS NULL)
        OR (decision IS NOT NULL AND decided_by_admin_id IS NOT NULL AND closed_at IS NOT NULL AND closed_at >= opened_at)
    ),
    CONSTRAINT cases_decision_shape_check CHECK (
        decision IS NULL
        OR (decision = 'VIOLATION' AND decision_category IS NOT NULL AND decision_action IS NOT NULL)
        OR (decision = 'NO_VIOLATION' AND decision_category IS NULL AND decision_action IS NULL)
    ),
    CONSTRAINT cases_action_target_check CHECK (
        decision_action IS NULL
        OR (decision_action = 'HIDE' AND target_kind IN ('COMMENT', 'FORUM_ENTRY', 'REVIEW', 'CHAT_MESSAGE'))
        OR (decision_action IN ('WARN', 'BAN') AND target_kind IN ('COMMENT', 'FORUM_ENTRY', 'BUYER', 'REVIEW', 'CHAT_MESSAGE'))
        OR (decision_action = 'TAKEDOWN' AND target_kind IN ('POST', 'COMMENT', 'FORUM_ENTRY', 'CHAT_MESSAGE', 'REVIEW_REPLY'))
    ),
    CONSTRAINT cases_measure_check CHECK ((decision_action IS NOT DISTINCT FROM 'TAKEDOWN') = (measure_id IS NOT NULL)),
    CONSTRAINT cases_reversed_by_check CHECK (reversed_by_admin_id > 0),
    CONSTRAINT cases_reversed_together_check CHECK (
        (reversed_at IS NULL AND reversed_by_admin_id IS NULL AND reverse_reason IS NULL)
        OR (reversed_at IS NOT NULL AND reversed_by_admin_id IS NOT NULL AND reverse_reason IS NOT NULL AND closed_at IS NOT NULL AND reversed_at >= closed_at)
    ),
    CONSTRAINT cases_reversed_hide_check CHECK (reversed_at IS NULL OR decision_action = 'HIDE'),
    CONSTRAINT cases_reverse_reason_check CHECK (reverse_reason IS NULL OR (char_length(reverse_reason) >= 1 AND char_length(reverse_reason) <= 500 AND reverse_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
CREATE UNIQUE INDEX cases_one_open_content ON moderation.cases (target_kind, content_id) WHERE closed_at IS NULL AND content_id IS NOT NULL;
CREATE UNIQUE INDEX cases_one_open_buyer ON moderation.cases (buyer_id) WHERE closed_at IS NULL AND buyer_id IS NOT NULL;
CREATE INDEX cases_open_by_time ON moderation.cases (opened_at, id) WHERE closed_at IS NULL;
CREATE INDEX cases_by_time ON moderation.cases (opened_at DESC, id DESC);
CREATE INDEX cases_closed_by_time ON moderation.cases (closed_at) WHERE closed_at IS NOT NULL;

CREATE TABLE moderation.case_snapshots (
    id             UUID        PRIMARY KEY,
    case_id        UUID        NOT NULL REFERENCES moderation.cases (id),
    content_sha256 BYTEA       NOT NULL,
    content        TEXT        NOT NULL,
    asset_ids      UUID[]      NOT NULL DEFAULT '{}',
    captured_at    TIMESTAMPTZ NOT NULL,
    CONSTRAINT case_snapshots_case_content_key UNIQUE (case_id, content_sha256),
    CONSTRAINT case_snapshots_sha256_check CHECK (octet_length(content_sha256) = 32),
    CONSTRAINT case_snapshots_content_check CHECK (char_length(content) >= 2)
);

CREATE TABLE moderation.reports (
    id                UUID        PRIMARY KEY,
    case_id           UUID        NOT NULL REFERENCES moderation.cases (id),
    reporter_buyer_id BIGINT,
    reporter_store_id BIGINT,
    category          TEXT        NOT NULL,
    description       TEXT,
    snapshot_id       UUID        REFERENCES moderation.case_snapshots (id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL,
    CONSTRAINT reports_case_reporter_key UNIQUE (case_id, reporter_buyer_id),
    CONSTRAINT reports_case_store_reporter_key UNIQUE (case_id, reporter_store_id),
    CONSTRAINT reports_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT reports_reporter_store_check CHECK (reporter_store_id > 0),
    CONSTRAINT reports_reporter_kind_check CHECK ((reporter_buyer_id IS NOT NULL) <> (reporter_store_id IS NOT NULL)),
    CONSTRAINT reports_category_check CHECK (category IN ('SPAM', 'HARASSMENT', 'HATE', 'SEXUAL_CONTENT', 'MISLEADING_INFO', 'FOOD_SAFETY', 'FRAUD', 'OTHER')),
    CONSTRAINT reports_description_check CHECK (description IS NULL OR (char_length(description) >= 1 AND char_length(description) <= 500 AND description !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'))
);
CREATE INDEX reports_case_time ON moderation.reports (case_id, created_at, id);
CREATE INDEX reports_reporter_time ON moderation.reports (reporter_buyer_id, created_at DESC, id DESC) WHERE reporter_buyer_id IS NOT NULL;
CREATE INDEX reports_store_reporter_time ON moderation.reports (reporter_store_id, created_at DESC, id DESC) WHERE reporter_store_id IS NOT NULL;
CREATE INDEX reports_snapshot ON moderation.reports (snapshot_id) WHERE snapshot_id IS NOT NULL;

CREATE TABLE moderation.report_requests (
    reporter_buyer_id BIGINT,
    reporter_store_id BIGINT,
    request_id        UUID   NOT NULL,
    report_id         UUID   NOT NULL REFERENCES moderation.reports (id),
    request_hash      BYTEA  NOT NULL,
    CONSTRAINT report_requests_buyer_key UNIQUE (reporter_buyer_id, request_id),
    CONSTRAINT report_requests_store_key UNIQUE (reporter_store_id, request_id),
    CONSTRAINT report_requests_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT report_requests_reporter_store_check CHECK (reporter_store_id > 0),
    CONSTRAINT report_requests_reporter_kind_check CHECK ((reporter_buyer_id IS NOT NULL) <> (reporter_store_id IS NOT NULL)),
    CONSTRAINT report_requests_hash_check CHECK (octet_length(request_hash) = 32)
);
CREATE INDEX report_requests_report ON moderation.report_requests (report_id);

CREATE TABLE moderation.reporter_daily_counts (
    reporter_buyer_id BIGINT,
    reporter_store_id BIGINT,
    day               DATE   NOT NULL,
    submitted         INT    NOT NULL,
    CONSTRAINT reporter_daily_counts_buyer_key UNIQUE (reporter_buyer_id),
    CONSTRAINT reporter_daily_counts_store_key UNIQUE (reporter_store_id),
    CONSTRAINT reporter_daily_counts_reporter_check CHECK (reporter_buyer_id > 0),
    CONSTRAINT reporter_daily_counts_reporter_store_check CHECK (reporter_store_id > 0),
    CONSTRAINT reporter_daily_counts_reporter_kind_check CHECK ((reporter_buyer_id IS NOT NULL) <> (reporter_store_id IS NOT NULL)),
    CONSTRAINT reporter_daily_counts_submitted_check CHECK (submitted >= 1 AND submitted <= 20)
);
