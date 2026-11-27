

CREATE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       (c.kyc_approved AND c.transfers_active) AS onboarding_complete,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code,
       c.kyc_approved,
       c.transfers_active
FROM seller.stores s
CROSS JOIN LATERAL (
    SELECT EXISTS (
               SELECT 1 FROM seller.kyc_applications a WHERE a.store_id = s.store_id AND a.status = 'APPROVED'
           ) AS kyc_approved,
           EXISTS (
               SELECT 1 FROM seller.payout_capabilities p WHERE p.store_id = s.store_id AND p.transfers_active
           ) AS transfers_active
) c;

CREATE TYPE seller.effective_measure AS (id UUID, account_id BIGINT);

CREATE FUNCTION seller.effective_measures(at TIMESTAMPTZ) RETURNS SETOF seller.effective_measure
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT m.id, m.account_id
    FROM seller.measures m
    WHERE m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at <= at AND (m.ends_at IS NULL OR m.ends_at > at);
END;

CREATE VIEW seller.identity_effect_measures AS
SELECT m.id, m.account_id
FROM seller.measures m
WHERE m.kind = 'SUSPENSION' OR (m.kind = 'TERMINATION' AND m.termination_ground <> 'NOTICE');

CREATE TYPE seller.measure_identity_source AS (measure_id UUID, account_id BIGINT, kind TEXT, key_id TEXT, hash BYTEA, created_at TIMESTAMPTZ);

CREATE FUNCTION seller.measure_identity_sources(at TIMESTAMPTZ) RETURNS SETOF seller.measure_identity_source
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT i.id, i.account_id, mi.kind, mi.key_id, mi.hash, NULL::timestamptz
    FROM seller.measure_identities mi
    JOIN seller.identity_effect_measures i ON i.id = mi.measure_id
    JOIN seller.effective_measures(at) e ON e.id = i.id
    UNION ALL
    SELECT i.id, i.account_id, CASE o.field WHEN 'TAX_ID' THEN 'TAX_ID' ELSE 'BANK_ACCOUNT' END, o.key_id, o.hash, o.created_at
    FROM seller.account_identity_hashes o
    JOIN seller.identity_effect_measures i ON i.account_id = o.account_id
    JOIN seller.effective_measures(at) e ON e.id = i.id;
END;

CREATE TYPE seller.measure_identity_hit AS (measure_id UUID, account_id BIGINT, field TEXT, created_at TIMESTAMPTZ);

CREATE FUNCTION seller.measure_identity_hits(at TIMESTAMPTZ) RETURNS SETOF seller.measure_identity_hit
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT s.measure_id, f.account_id, f.field, GREATEST(f.created_at, s.created_at)
    FROM seller.measure_identity_sources(at) s
    JOIN seller.account_identity_hashes f ON f.hash = s.hash AND f.key_id = s.key_id
    WHERE s.kind = CASE f.field WHEN 'TAX_ID' THEN 'TAX_ID' ELSE 'BANK_ACCOUNT' END;
END;

CREATE TYPE seller.pub_exposable_store AS (store_id BIGINT);

CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT f.store_id
    FROM seller.store_requirement_flags f
    WHERE f.active AND f.not_pending_deletion AND f.onboarding_complete AND f.has_store_name AND f.has_logo AND f.has_town_code
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = f.account_id);
END;

CREATE TYPE seller.review_item AS (
    kind TEXT,
    item_id TEXT,
    report_id UUID,
    case_id UUID,
    inquiry_id UUID,
    authority_order_id UUID,
    target_position SMALLINT,
    account_id BIGINT,
    measure_id UUID,
    request_id UUID,
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION seller.review_report_triage(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'REPORT_TRIAGE'::text, r.id::text, r.id, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, r.store_id, COALESCE(s.store_name, ''),
           r.created_at,
           r.created_at + CASE WHEN r.category = 'FOOD_SAFETY' THEN INTERVAL '4 hours' ELSE INTERVAL '72 hours' END
    FROM seller.reports r
    JOIN seller.stores s ON s.store_id = r.store_id
    WHERE r.case_id IS NULL AND r.outcome IS NULL;
END;

CREATE FUNCTION seller.review_case_decision(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_DECISION'::text, c.id::text, NULL::uuid, c.id, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, c.store_id, COALESCE(s.store_name, ''),
           k.start_at,
           k.start_at + CASE c.severity
               WHEN 'CRITICAL' THEN INTERVAL '24 hours'
               WHEN 'HIGH' THEN INTERVAL '72 hours'
               WHEN 'MEDIUM' THEN INTERVAL '168 hours'
               WHEN 'LOW' THEN INTERVAL '336 hours'
           END
    FROM seller.cases c
    JOIN seller.stores s ON s.store_id = c.store_id
    CROSS JOIN LATERAL (
        SELECT greatest(c.opened_at, max(i.reply_received_at), max(i.withdrawn_at) FILTER (WHERE i.withdraw_cause = 'ADMIN')) AS start_at
        FROM seller.case_inquiries i
        WHERE i.case_id = c.id
    ) k
    WHERE c.closed_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM seller.case_inquiries o WHERE o.case_id = c.id AND o.reply_received_at IS NULL AND o.withdrawn_at IS NULL);
END;

CREATE FUNCTION seller.review_case_inquiry_overdue(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_INQUIRY_OVERDUE'::text, i.id::text, NULL::uuid, i.case_id, i.id, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, i.store_id, COALESCE(s.store_name, ''),
           i.due_at, i.due_at + INTERVAL '24 hours'
    FROM seller.case_inquiries i
    JOIN seller.stores s ON s.store_id = i.store_id
    WHERE i.reply_received_at IS NULL AND i.withdrawn_at IS NULL AND i.due_at <= at;
END;

CREATE FUNCTION seller.review_authority_order(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'AUTHORITY_ORDER'::text, o.id::text, NULL::uuid, NULL::uuid, NULL::uuid, o.id, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::bigint, NULL::text,
           o.received_at, o.due_at
    FROM seller.authority_orders o
    WHERE o.completed_at IS NULL AND o.revoked_at IS NULL;
END;

CREATE FUNCTION seller.review_authority_target_notice(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'AUTHORITY_TARGET_NOTICE'::text, t.order_id::text || '/' || lpad(t.position::text, 5, '0'),
           NULL::uuid, NULL::uuid, NULL::uuid, t.order_id, t.position, t.account_id,
           NULL::uuid, NULL::uuid, NULL::uuid, t.store_id, CASE WHEN s.store_id IS NULL THEN NULL ELSE COALESCE(s.store_name, '') END,
           t.recheck_on::timestamp AT TIME ZONE 'Asia/Taipei',
           (t.recheck_on::timestamp AT TIME ZONE 'Asia/Taipei') + INTERVAL '72 hours'
    FROM seller.authority_order_targets t
    LEFT JOIN seller.stores s ON s.store_id = t.store_id
    WHERE t.notice = 'WITHHELD' AND t.recheck_on <= (at AT TIME ZONE 'Asia/Taipei')::date;
END;

CREATE FUNCTION seller.review_measure_appeal(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MEASURE_APPEAL'::text, q.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           q.measure_id, q.id, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           q.submitted_at, q.submitted_at + INTERVAL '168 hours'
    FROM seller.measure_requests q
    JOIN seller.measures m ON m.id = q.measure_id
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE q.state = 'PENDING' AND q.kind = 'APPEAL';
END;

CREATE FUNCTION seller.review_measure_lift_request(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MEASURE_LIFT_REQUEST'::text, q.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           q.measure_id, q.id, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           q.submitted_at, q.submitted_at + INTERVAL '168 hours'
    FROM seller.measure_requests q
    JOIN seller.measures m ON m.id = q.measure_id
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE q.state = 'PENDING' AND q.kind = 'REMEDIATION'
      AND EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.id = m.id);
END;

CREATE FUNCTION seller.review_measure_review(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MEASURE_REVIEW'::text, m.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           m.id, NULL::uuid, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           m.review_by - INTERVAL '24 hours', m.review_by
    FROM seller.measures m
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE m.basis = 'RISK' AND m.review_by IS NOT NULL AND m.review_by - INTERVAL '24 hours' <= at
      AND EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.id = m.id);
END;

CREATE FUNCTION seller.account_sanction_lifted_at(lifted_account_id BIGINT, at TIMESTAMPTZ) RETURNS TIMESTAMPTZ
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT max(least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END))
    FROM seller.measures m
    WHERE m.account_id = lifted_account_id AND m.kind IN ('SUSPENSION', 'TERMINATION')
      AND least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END) <= at
      AND least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END) > m.starts_at;
END;

CREATE FUNCTION seller.review_kyc_application(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'KYC_APPLICATION'::text, k.application_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, k.application_id, k.store_id, COALESCE(s.store_name, ''),
           greatest(k.submitted_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = k.store_id), seller.account_sanction_lifted_at(s.account_id, at)),
           greatest(k.submitted_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = k.store_id), seller.account_sanction_lifted_at(s.account_id, at)) + INTERVAL '168 hours'
    FROM seller.kyc_applications k
    JOIN seller.stores s ON s.store_id = k.store_id
    WHERE k.status = 'SUBMITTED' AND s.status = 'ACTIVE'
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = s.account_id);
END;

CREATE FUNCTION seller.review_shared_bank_account(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'SHARED_BANK_ACCOUNT'::text, p.measure_id::text || '/' || p.account_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, p.account_id,
           p.measure_id, NULL::uuid, NULL::uuid, st.store_id, COALESCE(st.store_name, ''),
           p.start_at, p.start_at + INTERVAL '168 hours'
    FROM (
        SELECT m.id AS measure_id, h.account_id, min(h.created_at) AS start_at
        FROM seller.effective_measures(at) e
        JOIN seller.measures m ON m.id = e.id
        JOIN seller.measure_identity_hits(at) h ON h.measure_id = m.id
        WHERE h.field = 'BANK_FINGERPRINT' AND h.account_id <> m.account_id
          AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) x WHERE x.account_id = h.account_id)
          AND NOT EXISTS (SELECT 1 FROM seller.measure_linked_reviews r WHERE r.measure_id = m.id AND r.account_id = h.account_id)
        GROUP BY m.id, m.created_at, h.account_id
        HAVING min(h.created_at) > m.created_at
    ) p
    CROSS JOIN LATERAL (
        SELECT s.store_id, s.store_name
        FROM seller.stores s
        WHERE s.account_id = p.account_id
        ORDER BY s.store_id DESC
        LIMIT 1
    ) st;
END;

CREATE TYPE seller.pub_review_item AS (
    kind TEXT,
    item_id TEXT,
    report_id UUID,
    case_id UUID,
    inquiry_id UUID,
    authority_order_id UUID,
    target_position SMALLINT,
    account_id BIGINT,
    measure_id UUID,
    request_id UUID,
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM seller.review_report_triage(at)
    UNION ALL
    SELECT * FROM seller.review_case_decision(at)
    UNION ALL
    SELECT * FROM seller.review_case_inquiry_overdue(at)
    UNION ALL
    SELECT * FROM seller.review_authority_order(at)
    UNION ALL
    SELECT * FROM seller.review_authority_target_notice(at)
    UNION ALL
    SELECT * FROM seller.review_measure_appeal(at)
    UNION ALL
    SELECT * FROM seller.review_measure_lift_request(at)
    UNION ALL
    SELECT * FROM seller.review_measure_review(at)
    UNION ALL
    SELECT * FROM seller.review_kyc_application(at)
    UNION ALL
    SELECT * FROM seller.review_shared_bank_account(at);
END;

CREATE TYPE seller.pub_content_takedown AS (measure_id UUID, content_kind TEXT, content_id UUID);

CREATE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN' AND c.measure_status = 'ENFORCED';
END;

CREATE FUNCTION seller.pub_lock_content_takedowns(p_measure_id UUID) RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql VOLATILE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.id = p_measure_id AND m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN'
    ORDER BY c.content_kind, c.content_id
    FOR SHARE OF m, c;
END;

CREATE TYPE seller.pub_retired_store AS (store_id BIGINT);

CREATE FUNCTION seller.pub_retired_stores() RETURNS SETOF seller.pub_retired_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT s.store_id
    FROM seller.stores s
    WHERE s.retired_at IS NOT NULL;
END;
