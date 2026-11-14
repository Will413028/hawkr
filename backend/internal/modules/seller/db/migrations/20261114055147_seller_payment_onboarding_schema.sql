-- Modify "account_identity_hashes" table
ALTER TABLE "seller"."account_identity_hashes" DROP CONSTRAINT "account_identity_hashes_field_check", ADD CONSTRAINT "account_identity_hashes_field_check" CHECK (field = ANY (ARRAY['TAX_ID'::text, 'BANK_FINGERPRINT'::text]));
-- Create index "account_identity_hashes_hash" to table: "account_identity_hashes"
CREATE INDEX "account_identity_hashes_hash" ON "seller"."account_identity_hashes" ("hash", "key_id");
-- Modify "agreement_acceptances" table
ALTER TABLE "seller"."agreement_acceptances" DROP CONSTRAINT "agreement_acceptances_kind_check", ADD CONSTRAINT "agreement_acceptances_kind_check" CHECK (kind = ANY (ARRAY['SELLER_TERMS'::text, 'KYC_PRIVACY_NOTICE'::text])), DROP CONSTRAINT "agreement_acceptances_source_check", ADD CONSTRAINT "agreement_acceptances_source_check" CHECK (((kind = 'SELLER_TERMS'::text) AND (submission_review_id IS NULL)) OR ((kind = 'KYC_PRIVACY_NOTICE'::text) AND (submission_review_id IS NOT NULL)));
-- Modify "kyc_applications" table
ALTER TABLE "seller"."kyc_applications" DROP CONSTRAINT "kyc_applications_bank_account_check", DROP CONSTRAINT "kyc_applications_bank_code_check", DROP CONSTRAINT "kyc_applications_bank_name_check", DROP CONSTRAINT "kyc_applications_birth_date_check", DROP CONSTRAINT "kyc_applications_english_name_check", DROP CONSTRAINT "kyc_applications_hash_length_check", DROP CONSTRAINT "kyc_applications_id_card_check", DROP CONSTRAINT "kyc_applications_issue_date_check", DROP CONSTRAINT "kyc_applications_issue_location_check", DROP CONSTRAINT "kyc_applications_issue_type_check", DROP CONSTRAINT "kyc_applications_real_name_check", DROP CONSTRAINT "kyc_applications_rep_id_card_check", DROP CONSTRAINT "kyc_applications_tax_id_hash_check", DROP CONSTRAINT "kyc_applications_complete_check", ADD CONSTRAINT "kyc_applications_complete_check" CHECK ((status <> ALL (ARRAY['SUBMITTED'::text, 'APPROVED'::text, 'REJECTED'::text])) OR ((submitted_at IS NOT NULL) AND ((pii_purged_at IS NOT NULL) OR ((seller_type = 'INDIVIDUAL'::text) OR ((seller_type = 'COMPANY'::text) AND (company_name IS NOT NULL) AND (tax_id IS NOT NULL)))))), DROP CONSTRAINT "kyc_applications_purged_check", ADD CONSTRAINT "kyc_applications_purged_check" CHECK ((pii_purged_at IS NULL) OR ((website_url IS NULL) AND (company_name IS NULL) AND (tax_id IS NULL) AND (company_register_date IS NULL) AND (company_town_code IS NULL) AND (company_address_detail IS NULL) AND (company_phone IS NULL))), DROP CONSTRAINT "kyc_applications_type_fields_check", ADD CONSTRAINT "kyc_applications_type_fields_check" CHECK ((seller_type = 'COMPANY'::text) OR ((company_name IS NULL) AND (tax_id IS NULL) AND (company_register_date IS NULL) AND (company_town_code IS NULL) AND (company_address_detail IS NULL) AND (company_phone IS NULL))), DROP COLUMN "real_name", DROP COLUMN "birth_date", DROP COLUMN "id_card_no_enc", DROP COLUMN "id_card_hash", DROP COLUMN "id_card_hash_key", DROP COLUMN "id_card_issue_date", DROP COLUMN "id_card_issue_type", DROP COLUMN "id_card_issue_location", DROP COLUMN "english_name", DROP COLUMN "bank_name", DROP COLUMN "bank_code", DROP COLUMN "bank_account_enc", DROP COLUMN "bank_account_hash", DROP COLUMN "bank_account_hash_key", DROP COLUMN "tax_id_hash", DROP COLUMN "tax_id_hash_key", DROP COLUMN "rep_id_card_no_enc", DROP COLUMN "rep_id_card_hash", DROP COLUMN "rep_id_card_hash_key";
-- Modify "kyc_documents" table
ALTER TABLE "seller"."kyc_documents" DROP CONSTRAINT "kyc_documents_slot_check", DROP CONSTRAINT "kyc_documents_pkey", DROP CONSTRAINT "kyc_documents_position_check", ADD CONSTRAINT "kyc_documents_position_check" CHECK (("position" >= 0) AND ("position" < 10)), DROP COLUMN "slot", ADD PRIMARY KEY ("application_id", "position");
-- Modify "kyc_review_issues" table
ALTER TABLE "seller"."kyc_review_issues" DROP CONSTRAINT "kyc_review_issues_asset_check", ADD CONSTRAINT "kyc_review_issues_asset_check" CHECK ((document_asset_id IS NULL) OR ((document_asset_id <> ''::text) AND (field = 'business.company_documents'::text))), DROP CONSTRAINT "kyc_review_issues_field_check", ADD CONSTRAINT "kyc_review_issues_field_check" CHECK (field = ANY (ARRAY['product_type'::text, 'website_url'::text, 'business.company_name'::text, 'business.tax_id'::text, 'business.company_register_date'::text, 'business.company_address'::text, 'business.company_phone'::text, 'business.company_documents'::text]));
-- Modify "kyc_reviews" table
ALTER TABLE "seller"."kyc_reviews" DROP CONSTRAINT "kyc_reviews_action_check", ADD CONSTRAINT "kyc_reviews_action_check" CHECK (action = ANY (ARRAY['SUBMIT'::text, 'APPROVE'::text, 'REJECT'::text, 'CANCEL'::text])), DROP CONSTRAINT "kyc_reviews_reviewer_check", ADD CONSTRAINT "kyc_reviews_reviewer_check" CHECK (((action = ANY (ARRAY['APPROVE'::text, 'REJECT'::text])) AND (reviewer_admin_id IS NOT NULL) AND (reviewer_admin_id > 0)) OR ((action = ANY (ARRAY['SUBMIT'::text, 'CANCEL'::text])) AND (reviewer_admin_id IS NULL)));
-- Modify "measure_identities" table
ALTER TABLE "seller"."measure_identities" DROP CONSTRAINT "measure_identities_kind_check", ADD CONSTRAINT "measure_identities_kind_check" CHECK (kind = ANY (ARRAY['TAX_ID'::text, 'BANK_ACCOUNT'::text]));
-- Modify "kyc_document_accesses" table
ALTER TABLE "seller"."kyc_document_accesses" DROP CONSTRAINT "kyc_document_accesses_slot_check", DROP CONSTRAINT "kyc_document_accesses_position_check", ADD CONSTRAINT "kyc_document_accesses_position_check" CHECK (("position" >= 0) AND ("position" < 10)), ADD CONSTRAINT "kyc_document_accesses_admin_check" CHECK (admin_id > 0), DROP COLUMN "reveal_id", DROP COLUMN "slot", ADD COLUMN "admin_id" bigint NOT NULL, ADD CONSTRAINT "kyc_document_accesses_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create "measure_linked_reviews" table
CREATE TABLE "seller"."measure_linked_reviews" (
  "measure_id" uuid NOT NULL,
  "account_id" bigint NOT NULL,
  "reviewed_at" timestamptz NOT NULL,
  "reviewed_by_admin_id" bigint NOT NULL,
  PRIMARY KEY ("measure_id", "account_id"),
  CONSTRAINT "measure_linked_reviews_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "measure_linked_reviews_admin_check" CHECK (reviewed_by_admin_id > 0)
);
-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_contact_name_check", ADD CONSTRAINT "stores_contact_name_check" CHECK ((char_length(contact_name) >= 1) AND (char_length(contact_name) <= 50) AND (contact_name !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)), DROP CONSTRAINT "stores_support_email_check", ADD CONSTRAINT "stores_support_email_check" CHECK ((char_length(support_email) <= 254) AND (support_email = lower(support_email)) AND (support_email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'::text));
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
-- Drop "kyc_identity_hashes" table
DROP TABLE "seller"."kyc_identity_hashes";
-- Drop "payuni_observations" table
DROP TABLE "seller"."payuni_observations";
-- Drop "payuni_registrations" table
DROP TABLE "seller"."payuni_registrations";
-- Drop "kyc_reveals" table
DROP TABLE "seller"."kyc_reveals";
