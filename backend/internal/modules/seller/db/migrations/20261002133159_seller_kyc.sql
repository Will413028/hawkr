-- Create "kyc_applications" table
CREATE TABLE "seller"."kyc_applications" (
  "application_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "status" text NOT NULL DEFAULT 'DRAFT',
  "version" bigint NOT NULL DEFAULT 1,
  "seller_type" text NOT NULL,
  "current_step" integer NOT NULL DEFAULT 0,
  "real_name" text NULL,
  "birth_date" date NULL,
  "id_card_no_enc" bytea NULL,
  "id_card_hash" bytea NULL,
  "id_card_hash_key" text NULL,
  "id_card_issue_date" date NULL,
  "id_card_issue_type" text NULL,
  "id_card_issue_location" text NULL,
  "english_name" text NULL,
  "bank_name" text NULL,
  "bank_code" text NULL,
  "bank_account_enc" bytea NULL,
  "bank_account_hash" bytea NULL,
  "bank_account_hash_key" text NULL,
  "product_type" text NULL,
  "is_processed_food" boolean NOT NULL DEFAULT false,
  "food_business_reg_no" text NULL,
  "website_url" text NULL,
  "contact_email" text NULL,
  "contact_phone" text NULL,
  "company_type" text NULL,
  "company_name" text NULL,
  "tax_id" text NULL,
  "tax_id_hash" bytea NULL,
  "tax_id_hash_key" text NULL,
  "company_register_date" date NULL,
  "company_town_code" text NULL,
  "company_address_detail" text NULL,
  "company_phone" text NULL,
  "rep_id_card_no_enc" bytea NULL,
  "rep_id_card_hash" bytea NULL,
  "rep_id_card_hash_key" text NULL,
  "submitted_at" timestamptz NULL,
  "decided_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("application_id"),
  CONSTRAINT "kyc_applications_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_applications_bank_account_check" CHECK (((bank_account_enc IS NULL) = (bank_account_hash IS NULL)) AND ((bank_account_hash IS NULL) = (bank_account_hash_key IS NULL))),
  CONSTRAINT "kyc_applications_bank_code_check" CHECK (bank_code ~ '^[0-9]{3}([0-9]{4})?$'::text),
  CONSTRAINT "kyc_applications_bank_name_check" CHECK ((char_length(bank_name) >= 1) AND (char_length(bank_name) <= 100)),
  CONSTRAINT "kyc_applications_birth_date_check" CHECK (birth_date >= '1900-01-01'::date),
  CONSTRAINT "kyc_applications_company_address_check" CHECK ((company_address_detail IS NULL) OR ((company_town_code IS NOT NULL) AND ((char_length(company_address_detail) >= 1) AND (char_length(company_address_detail) <= 255)))),
  CONSTRAINT "kyc_applications_company_name_check" CHECK ((char_length(company_name) >= 1) AND (char_length(company_name) <= 200)),
  CONSTRAINT "kyc_applications_company_phone_check" CHECK (company_phone ~ '^0[0-9]{8,9}$'::text),
  CONSTRAINT "kyc_applications_company_register_date_check" CHECK (company_register_date >= '1900-01-01'::date),
  CONSTRAINT "kyc_applications_company_town_check" CHECK (company_town_code ~ '^[0-9]{8}$'::text),
  CONSTRAINT "kyc_applications_company_type_check" CHECK ((char_length(company_type) >= 1) AND (char_length(company_type) <= 50)),
  CONSTRAINT "kyc_applications_complete_check" CHECK ((status <> ALL (ARRAY['SUBMITTED'::text, 'APPROVED'::text, 'REJECTED'::text])) OR ((submitted_at IS NOT NULL) AND (real_name IS NOT NULL) AND (english_name IS NOT NULL) AND (bank_name IS NOT NULL) AND (bank_code IS NOT NULL) AND (bank_account_enc IS NOT NULL) AND (contact_email IS NOT NULL) AND (contact_phone IS NOT NULL) AND (((seller_type = 'INDIVIDUAL'::text) AND (id_card_no_enc IS NOT NULL)) OR ((seller_type = 'COMPANY'::text) AND (company_name IS NOT NULL) AND (tax_id IS NOT NULL) AND (rep_id_card_no_enc IS NOT NULL))))),
  CONSTRAINT "kyc_applications_contact_email_check" CHECK ((char_length(contact_email) <= 254) AND (contact_email = lower(contact_email)) AND (contact_email ~ '^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$'::text)),
  CONSTRAINT "kyc_applications_contact_phone_check" CHECK (contact_phone ~ '^09[0-9]{8}$'::text),
  CONSTRAINT "kyc_applications_decided_check" CHECK ((status = ANY (ARRAY['APPROVED'::text, 'REJECTED'::text])) = (decided_at IS NOT NULL)),
  CONSTRAINT "kyc_applications_english_name_check" CHECK ((char_length(english_name) >= 1) AND (char_length(english_name) <= 50)),
  CONSTRAINT "kyc_applications_food_check" CHECK ((is_processed_food AND (food_business_reg_no IS NOT NULL) AND ((char_length(food_business_reg_no) >= 1) AND (char_length(food_business_reg_no) <= 50))) OR ((NOT is_processed_food) AND (food_business_reg_no IS NULL))),
  CONSTRAINT "kyc_applications_hash_length_check" CHECK ((octet_length(id_card_hash) = 32) AND (octet_length(rep_id_card_hash) = 32) AND (octet_length(bank_account_hash) = 32) AND (octet_length(tax_id_hash) = 32)),
  CONSTRAINT "kyc_applications_id_card_check" CHECK (((id_card_no_enc IS NULL) = (id_card_hash IS NULL)) AND ((id_card_hash IS NULL) = (id_card_hash_key IS NULL))),
  CONSTRAINT "kyc_applications_issue_date_check" CHECK (id_card_issue_date >= '1900-01-01'::date),
  CONSTRAINT "kyc_applications_issue_location_check" CHECK ((char_length(id_card_issue_location) >= 1) AND (char_length(id_card_issue_location) <= 100)),
  CONSTRAINT "kyc_applications_issue_type_check" CHECK ((char_length(id_card_issue_type) >= 1) AND (char_length(id_card_issue_type) <= 20)),
  CONSTRAINT "kyc_applications_product_type_check" CHECK ((char_length(product_type) >= 1) AND (char_length(product_type) <= 100)),
  CONSTRAINT "kyc_applications_real_name_check" CHECK ((char_length(real_name) >= 1) AND (char_length(real_name) <= 100) AND (real_name = btrim(real_name))),
  CONSTRAINT "kyc_applications_rep_id_card_check" CHECK (((rep_id_card_no_enc IS NULL) = (rep_id_card_hash IS NULL)) AND ((rep_id_card_hash IS NULL) = (rep_id_card_hash_key IS NULL))),
  CONSTRAINT "kyc_applications_seller_type_check" CHECK (seller_type = ANY (ARRAY['INDIVIDUAL'::text, 'COMPANY'::text])),
  CONSTRAINT "kyc_applications_status_check" CHECK (status = ANY (ARRAY['DRAFT'::text, 'SUBMITTED'::text, 'APPROVED'::text, 'REJECTED'::text, 'CANCELLED'::text])),
  CONSTRAINT "kyc_applications_step_check" CHECK ((current_step >= 0) AND (current_step <= 20)),
  CONSTRAINT "kyc_applications_tax_id_check" CHECK (tax_id ~ '^[0-9]{8}$'::text),
  CONSTRAINT "kyc_applications_tax_id_hash_check" CHECK (((tax_id IS NULL) = (tax_id_hash IS NULL)) AND ((tax_id_hash IS NULL) = (tax_id_hash_key IS NULL))),
  CONSTRAINT "kyc_applications_type_fields_check" CHECK (((seller_type = 'INDIVIDUAL'::text) AND (company_type IS NULL) AND (company_name IS NULL) AND (tax_id IS NULL) AND (company_register_date IS NULL) AND (company_town_code IS NULL) AND (company_address_detail IS NULL) AND (company_phone IS NULL) AND (rep_id_card_no_enc IS NULL)) OR ((seller_type = 'COMPANY'::text) AND (id_card_no_enc IS NULL))),
  CONSTRAINT "kyc_applications_version_check" CHECK (version >= 1),
  CONSTRAINT "kyc_applications_website_check" CHECK ((website_url ~ '^https?://[^/?#]+'::text) AND (char_length(website_url) <= 500))
);
-- Create index "kyc_applications_created" to table: "kyc_applications"
CREATE INDEX "kyc_applications_created" ON "seller"."kyc_applications" ("created_at" DESC, "application_id" DESC) WHERE (status <> 'DRAFT'::text);
-- Create index "kyc_applications_open" to table: "kyc_applications"
CREATE UNIQUE INDEX "kyc_applications_open" ON "seller"."kyc_applications" ("store_id") WHERE (status <> 'CANCELLED'::text);
-- Create index "kyc_applications_status" to table: "kyc_applications"
CREATE INDEX "kyc_applications_status" ON "seller"."kyc_applications" ("status", "created_at" DESC, "application_id" DESC) WHERE (status <> 'DRAFT'::text);
-- Create index "kyc_applications_store" to table: "kyc_applications"
CREATE INDEX "kyc_applications_store" ON "seller"."kyc_applications" ("store_id", "created_at" DESC, "application_id" DESC);
-- Create "kyc_documents" table
CREATE TABLE "seller"."kyc_documents" (
  "application_id" uuid NOT NULL,
  "slot" text NOT NULL,
  "position" integer NOT NULL DEFAULT 0,
  "asset_id" text NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("application_id", "slot", "position"),
  CONSTRAINT "kyc_documents_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_documents_asset_check" CHECK ((asset_id <> ''::text) AND (object_key <> ''::text)),
  CONSTRAINT "kyc_documents_position_check" CHECK (("position" >= 0) AND ("position" < 10) AND ((slot = 'COMPANY_DOCUMENT'::text) OR ("position" = 0))),
  CONSTRAINT "kyc_documents_slot_check" CHECK (slot = ANY (ARRAY['ID_CARD_FRONT'::text, 'ID_CARD_BACK'::text, 'BANK_BOOK'::text, 'COMPANY_DOCUMENT'::text]))
);
-- Create "kyc_reveals" table
CREATE TABLE "seller"."kyc_reveals" (
  "reveal_id" uuid NOT NULL,
  "application_id" uuid NOT NULL,
  "admin_id" bigint NOT NULL,
  "reason" text NOT NULL,
  "revealed_at" timestamptz NOT NULL,
  PRIMARY KEY ("reveal_id"),
  CONSTRAINT "kyc_reveals_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_reveals_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "kyc_reveals_reason_check" CHECK ((char_length(reason) >= 1) AND (char_length(reason) <= 1000))
);
-- Create index "kyc_reveals_application" to table: "kyc_reveals"
CREATE INDEX "kyc_reveals_application" ON "seller"."kyc_reveals" ("application_id", "revealed_at");
-- Create "kyc_reviews" table
CREATE TABLE "seller"."kyc_reviews" (
  "review_id" uuid NOT NULL,
  "application_id" uuid NOT NULL,
  "action" text NOT NULL,
  "reviewer_admin_id" bigint NULL,
  "reviewer_name" text NULL,
  "reason" text NULL,
  "required_fields" text[] NOT NULL DEFAULT '{}',
  "decided_at" timestamptz NOT NULL,
  PRIMARY KEY ("review_id"),
  CONSTRAINT "kyc_reviews_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_reviews_action_check" CHECK (action = ANY (ARRAY['SUBMIT'::text, 'APPROVE'::text, 'REJECT'::text, 'CANCEL'::text])),
  CONSTRAINT "kyc_reviews_reason_check" CHECK ((char_length(reason) >= 1) AND (char_length(reason) <= 1000)),
  CONSTRAINT "kyc_reviews_reject_reason_check" CHECK ((action <> 'REJECT'::text) OR (reason IS NOT NULL)),
  CONSTRAINT "kyc_reviews_required_fields_check" CHECK ((action = 'REJECT'::text) OR (required_fields = '{}'::text[])),
  CONSTRAINT "kyc_reviews_reviewer_check" CHECK (((action = ANY (ARRAY['APPROVE'::text, 'REJECT'::text])) AND (reviewer_admin_id IS NOT NULL) AND (reviewer_admin_id > 0) AND (reviewer_name IS NOT NULL)) OR ((action = ANY (ARRAY['SUBMIT'::text, 'CANCEL'::text])) AND (reviewer_admin_id IS NULL) AND (reviewer_name IS NULL))),
  CONSTRAINT "kyc_reviews_reviewer_name_check" CHECK ((char_length(reviewer_name) >= 1) AND (char_length(reviewer_name) <= 100))
);
-- Create index "kyc_reviews_application" to table: "kyc_reviews"
CREATE INDEX "kyc_reviews_application" ON "seller"."kyc_reviews" ("application_id", "decided_at", "review_id");
-- Create "suspension_identities" table
CREATE TABLE "seller"."suspension_identities" (
  "suspension_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "key_id" text NOT NULL,
  "hash" bytea NOT NULL,
  PRIMARY KEY ("suspension_id", "kind", "hash"),
  CONSTRAINT "suspension_identities_suspension_id_fkey" FOREIGN KEY ("suspension_id") REFERENCES "seller"."suspensions" ("suspension_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "suspension_identities_hash_check" CHECK (octet_length(hash) = 32),
  CONSTRAINT "suspension_identities_key_check" CHECK (key_id <> ''::text),
  CONSTRAINT "suspension_identities_kind_check" CHECK (kind = ANY (ARRAY['ID_CARD'::text, 'TAX_ID'::text, 'BANK_ACCOUNT'::text]))
);
-- Create index "suspension_identities_hash" to table: "suspension_identities"
CREATE INDEX "suspension_identities_hash" ON "seller"."suspension_identities" ("hash");
