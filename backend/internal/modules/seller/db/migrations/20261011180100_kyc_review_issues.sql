-- Modify "kyc_reviews" table
ALTER TABLE "seller"."kyc_reviews" DROP CONSTRAINT "kyc_reviews_required_fields_check", ADD CONSTRAINT "kyc_reviews_internal_note_check" CHECK ((internal_note IS NULL) OR ((action = 'REJECT'::text) AND ((char_length(internal_note) >= 1) AND (char_length(internal_note) <= 1000)))), DROP COLUMN "required_fields", ADD COLUMN "internal_note" text NULL;
-- Create "kyc_review_issues" table
CREATE TABLE "seller"."kyc_review_issues" (
  "issue_id" uuid NOT NULL,
  "review_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "field" text NOT NULL,
  "document_asset_id" text NULL,
  "code" text NOT NULL,
  "message" text NOT NULL,
  "resolved_at" timestamptz NULL,
  PRIMARY KEY ("issue_id"),
  CONSTRAINT "kyc_review_issues_position_key" UNIQUE ("review_id", "position"),
  CONSTRAINT "kyc_review_issues_review_id_fkey" FOREIGN KEY ("review_id") REFERENCES "seller"."kyc_reviews" ("review_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_review_issues_asset_check" CHECK ((document_asset_id IS NULL) OR ((document_asset_id <> ''::text) AND (field = ANY (ARRAY['id_card_front'::text, 'id_card_back'::text, 'bank_book'::text, 'business.company_documents'::text])))),
  CONSTRAINT "kyc_review_issues_code_check" CHECK (code = ANY (ARRAY['UNREADABLE'::text, 'MISMATCH'::text, 'EXPIRED'::text, 'MISSING'::text, 'INVALID'::text, 'OTHER'::text])),
  CONSTRAINT "kyc_review_issues_field_check" CHECK (field = ANY (ARRAY['real_name'::text, 'birth_date'::text, 'id_card_issue_date'::text, 'id_card_issue_type'::text, 'id_card_issue_location'::text, 'english_name'::text, 'bank_name'::text, 'bank_code'::text, 'bank_account'::text, 'product_type'::text, 'food_business'::text, 'website_url'::text, 'id_card_front'::text, 'id_card_back'::text, 'bank_book'::text, 'individual.id_card_no'::text, 'business.company_name'::text, 'business.tax_id'::text, 'business.company_register_date'::text, 'business.company_address'::text, 'business.company_phone'::text, 'business.rep_id_card_no'::text, 'business.company_documents'::text])),
  CONSTRAINT "kyc_review_issues_message_check" CHECK ((char_length(message) >= 1) AND (char_length(message) <= 500)),
  CONSTRAINT "kyc_review_issues_position_check" CHECK ("position" >= 0)
);
