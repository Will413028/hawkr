-- 食品業者登錄拆成獨立的審核資源（food_registrations）：店家與 KYC 申請不再有食品欄位，KYC 退回問題不能再指向 food_business。
-- 店家上既有的字號按定義沒有核對過，不搬，只用 NOTICE 列出受影響的 store_id；退回問題裡有 food_business 的列無法保留（新的欄位
-- CHECK 不允許），先核對，有就整個 migration 失敗並列出 ID，請先人工處理後重跑（先例：20261012214337）。
DO $$
DECLARE
  issue_rows text;
  stores_with_number text;
BEGIN
  SELECT string_agg(i.issue_id::text, ', ' ORDER BY i.issue_id) INTO issue_rows
  FROM "seller"."kyc_review_issues" i WHERE i.field = 'food_business';
  IF issue_rows IS NOT NULL THEN
    RAISE EXCEPTION 'kyc_review_issues 有指向 food_business 的退回問題，欄位已從 KYC 移除，請先人工處理這些列後重跑：%', issue_rows;
  END IF;
  SELECT string_agg(s.store_id::text, ', ' ORDER BY s.store_id) INTO stores_with_number
  FROM "seller"."stores" s WHERE s.food_business_reg_no IS NOT NULL;
  IF stores_with_number IS NOT NULL THEN
    RAISE NOTICE '這些店家的食品業者登錄字號沒有經過核對，不搬到 food_registrations，需要的話請賣家重新送審：%', stores_with_number;
  END IF;
END
$$;
-- Modify "kyc_applications" table
ALTER TABLE "seller"."kyc_applications" DROP CONSTRAINT "kyc_applications_food_check", DROP COLUMN "is_processed_food", DROP COLUMN "food_business_reg_no";
-- Modify "kyc_review_issues" table
ALTER TABLE "seller"."kyc_review_issues" DROP CONSTRAINT "kyc_review_issues_field_check", ADD CONSTRAINT "kyc_review_issues_field_check" CHECK (field = ANY (ARRAY['real_name'::text, 'birth_date'::text, 'id_card_issue_date'::text, 'id_card_issue_type'::text, 'id_card_issue_location'::text, 'english_name'::text, 'bank_name'::text, 'bank_code'::text, 'bank_account'::text, 'product_type'::text, 'website_url'::text, 'id_card_front'::text, 'id_card_back'::text, 'bank_book'::text, 'individual.id_card_no'::text, 'business.company_name'::text, 'business.tax_id'::text, 'business.company_register_date'::text, 'business.company_address'::text, 'business.company_phone'::text, 'business.rep_id_card_no'::text, 'business.company_documents'::text]));
-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_food_registration_check", DROP COLUMN "is_processed_food", DROP COLUMN "food_business_reg_no";
-- Create "food_registrations" table
CREATE TABLE "seller"."food_registrations" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "registration_number" text NULL,
  "state" text NOT NULL,
  "submitted_at" timestamptz NOT NULL,
  "decided_by_admin_id" bigint NULL,
  "decided_at" timestamptz NULL,
  "reject_reason" text NULL,
  "internal_note" text NULL,
  "ended_at" timestamptz NULL,
  "end_cause" text NULL,
  "end_reason" text NULL,
  "ended_by_admin_id" bigint NULL,
  "pii_purged_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "food_registrations_store_fkey" FOREIGN KEY ("store_id", "account_id") REFERENCES "seller"."stores" ("store_id", "account_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "food_registrations_approved_number_check" CHECK ((state <> 'APPROVED'::text) OR (registration_number IS NOT NULL)),
  CONSTRAINT "food_registrations_decision_check" CHECK (((state = ANY (ARRAY['PENDING'::text, 'WITHDRAWN'::text])) AND (decided_by_admin_id IS NULL) AND (decided_at IS NULL) AND (reject_reason IS NULL) AND (internal_note IS NULL)) OR ((state = ANY (ARRAY['APPROVED'::text, 'ENDED'::text])) AND (decided_by_admin_id IS NOT NULL) AND (decided_by_admin_id > 0) AND (decided_at IS NOT NULL) AND (decided_at >= submitted_at) AND (reject_reason IS NULL)) OR ((state = 'REJECTED'::text) AND (decided_by_admin_id IS NOT NULL) AND (decided_by_admin_id > 0) AND (decided_at IS NOT NULL) AND (decided_at >= submitted_at) AND ((reject_reason IS NOT NULL) OR (pii_purged_at IS NOT NULL)))),
  CONSTRAINT "food_registrations_end_cause_check" CHECK (end_cause = ANY (ARRAY['SUPERSEDED'::text, 'CLEARED'::text, 'REVOKED'::text, 'STORE_RETIRED'::text])),
  CONSTRAINT "food_registrations_end_check" CHECK (((state = ANY (ARRAY['PENDING'::text, 'APPROVED'::text, 'REJECTED'::text])) AND (ended_at IS NULL) AND (end_cause IS NULL)) OR ((state = 'WITHDRAWN'::text) AND (ended_at IS NOT NULL) AND (ended_at >= submitted_at) AND (end_cause IS NULL)) OR ((state = 'ENDED'::text) AND (ended_at IS NOT NULL) AND (ended_at >= decided_at) AND (end_cause IS NOT NULL))),
  CONSTRAINT "food_registrations_end_reason_check" CHECK ((char_length(end_reason) >= 1) AND (char_length(end_reason) <= 1000)),
  CONSTRAINT "food_registrations_internal_note_check" CHECK ((char_length(internal_note) >= 1) AND (char_length(internal_note) <= 1000)),
  CONSTRAINT "food_registrations_number_check" CHECK ((registration_number = btrim(registration_number)) AND ((char_length(registration_number) >= 1) AND (char_length(registration_number) <= 50))),
  CONSTRAINT "food_registrations_purged_check" CHECK (((pii_purged_at IS NULL) AND (registration_number IS NOT NULL)) OR ((pii_purged_at IS NOT NULL) AND (registration_number IS NULL) AND (reject_reason IS NULL) AND (end_reason IS NULL) AND (internal_note IS NULL))),
  CONSTRAINT "food_registrations_reject_reason_check" CHECK ((char_length(reject_reason) >= 1) AND (char_length(reject_reason) <= 1000)),
  CONSTRAINT "food_registrations_revoke_check" CHECK (((end_cause = 'REVOKED'::text) AND (ended_by_admin_id IS NOT NULL) AND (ended_by_admin_id > 0) AND ((end_reason IS NOT NULL) OR (pii_purged_at IS NOT NULL))) OR ((end_cause IS DISTINCT FROM 'REVOKED'::text) AND (end_reason IS NULL) AND (ended_by_admin_id IS NULL))),
  CONSTRAINT "food_registrations_state_check" CHECK (state = ANY (ARRAY['PENDING'::text, 'APPROVED'::text, 'REJECTED'::text, 'WITHDRAWN'::text, 'ENDED'::text]))
);
-- Create index "food_registrations_inbox" to table: "food_registrations"
CREATE INDEX "food_registrations_inbox" ON "seller"."food_registrations" ("submitted_at", "id") WHERE (state = 'PENDING'::text);
-- Create index "food_registrations_one_approved" to table: "food_registrations"
CREATE UNIQUE INDEX "food_registrations_one_approved" ON "seller"."food_registrations" ("store_id") WHERE (state = 'APPROVED'::text);
-- Create index "food_registrations_one_pending" to table: "food_registrations"
CREATE UNIQUE INDEX "food_registrations_one_pending" ON "seller"."food_registrations" ("store_id") WHERE (state = 'PENDING'::text);
-- Create index "food_registrations_store" to table: "food_registrations"
CREATE INDEX "food_registrations_store" ON "seller"."food_registrations" ("store_id", "submitted_at" DESC, "id" DESC);
-- Modify "agreement_acceptances" table
ALTER TABLE "seller"."agreement_acceptances" DROP CONSTRAINT "agreement_acceptances_submission_check", DROP CONSTRAINT "agreement_acceptances_kind_check", ADD CONSTRAINT "agreement_acceptances_kind_check" CHECK (kind = ANY (ARRAY['SELLER_TERMS'::text, 'KYC_PRIVACY_NOTICE'::text, 'PAYUNI_TERMS'::text, 'FOOD_REGISTRATION_NOTICE'::text])), ADD CONSTRAINT "agreement_acceptances_source_check" CHECK (((kind = 'SELLER_TERMS'::text) AND (submission_review_id IS NULL) AND (food_registration_id IS NULL)) OR ((kind = ANY (ARRAY['KYC_PRIVACY_NOTICE'::text, 'PAYUNI_TERMS'::text])) AND (submission_review_id IS NOT NULL) AND (food_registration_id IS NULL)) OR ((kind = 'FOOD_REGISTRATION_NOTICE'::text) AND (submission_review_id IS NULL) AND (food_registration_id IS NOT NULL))), ADD COLUMN "food_registration_id" uuid NULL, ADD CONSTRAINT "agreement_acceptances_food_registration_id_fkey" FOREIGN KEY ("food_registration_id") REFERENCES "seller"."food_registrations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE;
-- Create index "agreement_acceptances_food_registration" to table: "agreement_acceptances"
CREATE INDEX "agreement_acceptances_food_registration" ON "seller"."agreement_acceptances" ("food_registration_id") WHERE (food_registration_id IS NOT NULL);
