-- 後台對外的 PayUni 入口退場（Stripe 計畫 D14、步驟 8a）：管理員重新綁定刪除，審核紀錄不再有 REBIND，後台待辦不再有 PAYUNI_BIND_RETRY。
-- 系統未上線，REBIND 的既有列直接刪（不轉換）；payuni_* 兩張表本次不動。
-- 手寫的 migration（單體規則 6）；定義與 schema.sql、objects.sql 相同（TestSchemaFileMatchesMigrations），註解見那兩個檔。
DELETE FROM seller.kyc_reviews WHERE action = 'REBIND';

ALTER TABLE "seller"."kyc_reviews" DROP CONSTRAINT "kyc_reviews_action_check", DROP CONSTRAINT "kyc_reviews_reviewer_check",
  ADD CONSTRAINT "kyc_reviews_action_check" CHECK (action = ANY (ARRAY['SUBMIT'::text, 'APPROVE'::text, 'REJECT'::text, 'CANCEL'::text])),
  ADD CONSTRAINT "kyc_reviews_reviewer_check" CHECK (
    ((action = ANY (ARRAY['APPROVE'::text, 'REJECT'::text])) AND reviewer_admin_id IS NOT NULL AND reviewer_admin_id > 0) OR
    ((action = ANY (ARRAY['SUBMIT'::text, 'CANCEL'::text])) AND reviewer_admin_id IS NULL)
  );

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
    SELECT * FROM seller.review_kyc_application(at);
END;

DROP FUNCTION seller.review_payuni_bind_retry(TIMESTAMPTZ);
