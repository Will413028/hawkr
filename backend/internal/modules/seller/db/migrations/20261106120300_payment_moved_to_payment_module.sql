-- 收款帳戶搬到 payment 模組（Stripe 計畫 D28、D29，步驟 7d′）：payment_accounts 與 webhook inbox payment_provider_events 刪除（帳戶列由 payment.accounts
-- 擁有，inbox 由 platform 的 inbox.events 提供；系統未上線，不搬資料）。開通閘門改讀 seller 自存的副本 payout_capabilities（payment 的
-- PayoutAccountChanged 由同 transaction 的訂閱者寫入）；後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED 改由 payment 發布。
-- 手寫的 migration（單體規則 6）；定義與 schema.sql、objects.sql 相同（TestSchemaFileMatchesMigrations），註解見那兩個檔。
-- Create "payout_capabilities" table
CREATE TABLE "seller"."payout_capabilities" (
  "store_id" bigint NOT NULL,
  "provider" text NOT NULL,
  "transfers_active" boolean NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "payout_capabilities_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "payout_capabilities_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text)
);

CREATE OR REPLACE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       EXISTS (
           SELECT 1
           FROM seller.kyc_applications a
           JOIN seller.payout_capabilities p ON p.store_id = a.store_id
           WHERE a.store_id = s.store_id AND a.status = 'APPROVED' AND p.transfers_active
       ) AS onboarding_complete,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code
FROM seller.stores s;

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
    SELECT * FROM seller.review_payuni_bind_retry(at);
END;

DROP FUNCTION seller.review_payment_account_create_failed(TIMESTAMPTZ);
DROP TABLE seller.payment_accounts;
DROP TABLE seller.payment_provider_events;
