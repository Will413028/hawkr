-- Cleanup: the field leaves the contract; rows that still name it lose it before the CHECKs tighten.
UPDATE "seller"."kyc_registration_checks"
SET "mismatched_fields" = array_remove("mismatched_fields", 'business.company_type'),
    "unverifiable_fields" = array_remove("unverifiable_fields", 'business.company_type')
WHERE 'business.company_type' = ANY("mismatched_fields") OR 'business.company_type' = ANY("unverifiable_fields");
UPDATE "seller"."kyc_reviews"
SET "required_fields" = array_remove("required_fields", 'business.company_type')
WHERE 'business.company_type' = ANY("required_fields");
-- Modify "kyc_applications" table
ALTER TABLE "seller"."kyc_applications" DROP CONSTRAINT "kyc_applications_company_type_check", DROP CONSTRAINT "kyc_applications_type_fields_check", ADD CONSTRAINT "kyc_applications_type_fields_check" CHECK (((seller_type = 'INDIVIDUAL'::text) AND (company_name IS NULL) AND (tax_id IS NULL) AND (company_register_date IS NULL) AND (company_town_code IS NULL) AND (company_address_detail IS NULL) AND (company_phone IS NULL) AND (rep_id_card_no_enc IS NULL)) OR ((seller_type = 'COMPANY'::text) AND (id_card_no_enc IS NULL))), DROP COLUMN "company_type";
-- Modify "kyc_registration_checks" table
ALTER TABLE "seller"."kyc_registration_checks" DROP CONSTRAINT "kyc_registration_checks_fields_check", ADD CONSTRAINT "kyc_registration_checks_fields_check" CHECK ((mismatched_fields <@ ARRAY['business.company_name'::text, 'business.company_address'::text, 'business.company_register_date'::text]) AND (unverifiable_fields <@ ARRAY['business.company_name'::text, 'business.company_address'::text, 'business.company_register_date'::text]) AND (NOT (mismatched_fields && unverifiable_fields)));
