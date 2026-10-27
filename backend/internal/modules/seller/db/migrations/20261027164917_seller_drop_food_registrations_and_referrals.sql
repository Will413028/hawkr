-- 刪除食品業者登錄與食安案件的轉報（ADR 2026-10-27-general-ecommerce-scope-cut D3）：刪 food_registrations、case_referrals、
-- case_referral_replies、同意紀錄的 FOOD_REGISTRATION_NOTICE 與 food_registration_id、cases 只給轉報用的複合唯一鍵與索引，以及待辦的
-- CASE_FDA_REFERRAL、FOOD_REGISTRATION 兩種。review_item 與 pub_review_item 少了 food_registration_id，複合型別不能刪欄位後保留依賴，
-- 所以依賴它們的函式全部 drop 再建。手寫（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），
-- 註解見 objects.sql。系統從未上線，沒有要保留的資料。
DROP FUNCTION seller.pub_review_items(timestamptz);
DROP FUNCTION seller.review_report_triage(timestamptz);
DROP FUNCTION seller.review_case_decision(timestamptz);
DROP FUNCTION seller.review_case_inquiry_overdue(timestamptz);
DROP FUNCTION seller.review_case_fda_referral(timestamptz);
DROP FUNCTION seller.review_authority_order(timestamptz);
DROP FUNCTION seller.review_authority_target_notice(timestamptz);
DROP FUNCTION seller.review_measure_appeal(timestamptz);
DROP FUNCTION seller.review_measure_lift_request(timestamptz);
DROP FUNCTION seller.review_measure_review(timestamptz);
DROP FUNCTION seller.review_food_registration(timestamptz);
DROP FUNCTION seller.review_kyc_application(timestamptz);
DROP FUNCTION seller.review_payuni_bind_retry(timestamptz);
DROP TYPE seller.pub_review_item;
DROP TYPE seller.review_item;

DELETE FROM seller.agreement_acceptances WHERE kind = 'FOOD_REGISTRATION_NOTICE';
ALTER TABLE seller.agreement_acceptances DROP CONSTRAINT agreement_acceptances_kind_check;
ALTER TABLE seller.agreement_acceptances DROP CONSTRAINT agreement_acceptances_source_check;
DROP INDEX seller.agreement_acceptances_food_registration;
ALTER TABLE seller.agreement_acceptances DROP COLUMN food_registration_id;
ALTER TABLE seller.agreement_acceptances ADD CONSTRAINT agreement_acceptances_kind_check CHECK (kind IN ('SELLER_TERMS', 'KYC_PRIVACY_NOTICE', 'PAYUNI_TERMS'));
ALTER TABLE seller.agreement_acceptances ADD CONSTRAINT agreement_acceptances_source_check CHECK (
    (kind = 'SELLER_TERMS' AND submission_review_id IS NULL) OR
    (kind IN ('KYC_PRIVACY_NOTICE', 'PAYUNI_TERMS') AND submission_review_id IS NOT NULL)
);
DROP TABLE seller.food_registrations;

DROP TABLE seller.case_referral_replies;
DROP TABLE seller.case_referrals;
DROP INDEX seller.cases_decided_food_safety;
ALTER TABLE seller.cases DROP CONSTRAINT cases_category_key;

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

CREATE FUNCTION seller.review_payuni_bind_retry(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'PAYUNI_BIND_RETRY'::text, k.application_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, k.application_id, r.store_id, COALESCE(s.store_name, ''),
           r.bind_failed_at, r.bind_failed_at + INTERVAL '24 hours'
    FROM seller.payuni_registrations r
    JOIN seller.stores s ON s.store_id = r.store_id
    JOIN seller.kyc_applications k ON k.application_id = r.application_id
    WHERE r.bind_status = 'FAILED' AND r.bind_error_code = 'PAYUNI_UNAVAILABLE' AND s.status = 'ACTIVE' AND k.status = 'APPROVED';
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
    SELECT * FROM seller.review_payuni_bind_retry(at);
END;
