-- seller.review_payuni_bind_retry(at)（PAYUNI_BIND_RETRY 待辦，step10e-5a-design.md F16）與把它加進 pub_review_items 的 UNION ALL（12 支）。
-- 手寫（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。
-- PAYUNI_BIND_RETRY 重新綁定 PayUni：綁定重試用完（bind_error_code 是 PAYUNI_UNAVAILABLE）、店家營運中、註冊的申請是已核准的（賣家把它改回草稿時
-- 不列：重新核准之後會自動重新綁定）。其他綁定失敗（資料不完整、被永久拒絕）賣家被告知找客服，由客服接手，不進待辦。開始是綁定失敗的時刻
-- （bind_failed_at；不用 updated_at：失敗期間店家改地址會推後它），期限 24 小時（上線前的起始值）。
CREATE FUNCTION seller.review_payuni_bind_retry(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'PAYUNI_BIND_RETRY'::text, k.application_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, k.application_id, r.store_id, COALESCE(s.store_name, ''),
           r.bind_failed_at, r.bind_failed_at + INTERVAL '24 hours'
    FROM seller.payuni_registrations r
    JOIN seller.stores s ON s.store_id = r.store_id
    JOIN seller.kyc_applications k ON k.application_id = r.application_id
    WHERE r.bind_status = 'FAILED' AND r.bind_error_code = 'PAYUNI_UNAVAILABLE' AND s.status = 'ACTIVE' AND k.status = 'APPROVED';
END;

DROP FUNCTION seller.pub_review_items(timestamptz);

CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM seller.review_report_triage(at)
    UNION ALL
    SELECT * FROM seller.review_case_decision(at)
    UNION ALL
    SELECT * FROM seller.review_case_inquiry_overdue(at)
    UNION ALL
    SELECT * FROM seller.review_case_fda_referral(at)
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
    SELECT * FROM seller.review_food_registration(at)
    UNION ALL
    SELECT * FROM seller.review_kyc_application(at)
    UNION ALL
    SELECT * FROM seller.review_payuni_bind_retry(at);
END;
