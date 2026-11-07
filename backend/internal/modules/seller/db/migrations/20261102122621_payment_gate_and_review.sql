-- 開通閘門改看收款帳戶（Stripe 計畫 D10、D20）：store_requirement_flags 的 kyc_approved 改名 onboarding_complete，條件是平台審核核准
-- 而且 payment_accounts.transfers_active。依賴它的 pub_exposable_stores 一併重建；後台待辦加 PAYMENT_ACCOUNT_CREATE_FAILED。
-- 手寫的 migration（單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。
DROP FUNCTION seller.pub_exposable_stores(TIMESTAMPTZ);
DROP VIEW seller.store_requirement_flags;

CREATE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       EXISTS (
           SELECT 1
           FROM seller.kyc_applications a
           JOIN seller.payment_accounts p ON p.store_id = a.store_id
           WHERE a.store_id = s.store_id AND a.status = 'APPROVED' AND p.transfers_active
       ) AS onboarding_complete,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code
FROM seller.stores s;

CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT f.store_id
    FROM seller.store_requirement_flags f
    WHERE f.active AND f.not_pending_deletion AND f.onboarding_complete AND f.has_store_name AND f.has_logo AND f.has_town_code
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = f.account_id);
END;

CREATE FUNCTION seller.review_payment_account_create_failed(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'PAYMENT_ACCOUNT_CREATE_FAILED'::text, p.store_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, p.store_id, COALESCE(s.store_name, ''),
           p.create_failed_at, p.create_failed_at + INTERVAL '24 hours'
    FROM seller.payment_accounts p
    JOIN seller.stores s ON s.store_id = p.store_id
    WHERE p.create_failed_at IS NOT NULL AND s.status = 'ACTIVE';
END;

CREATE OR REPLACE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
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
    SELECT * FROM seller.review_payuni_bind_retry(at)
    UNION ALL
    SELECT * FROM seller.review_payment_account_create_failed(at);
END;
