-- 依 objects.sql 重建 drop_objects 拿掉的 view 與函式，並建立新增的物件。Atlas 社群版略過函式、view 與複合型別，這支 migration 是手寫的（單體規則 6）；
-- 定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。

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

CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT f.store_id
    FROM seller.store_requirement_flags f
    WHERE f.active AND f.not_pending_deletion AND f.onboarding_complete AND f.has_store_name AND f.has_logo AND f.has_town_code
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = f.account_id);
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
