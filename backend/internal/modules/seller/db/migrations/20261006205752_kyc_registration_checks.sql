-- Modify "kyc_applications" table
ALTER TABLE "seller"."kyc_applications" ADD COLUMN "submission_seq" bigint NOT NULL DEFAULT 0;
-- Backfill: applications that were already submitted count as one submission.
UPDATE "seller"."kyc_applications" SET "submission_seq" = 1 WHERE "status" IN ('SUBMITTED', 'APPROVED', 'REJECTED');
ALTER TABLE "seller"."kyc_applications" ADD CONSTRAINT "kyc_applications_submission_seq_check" CHECK ((submission_seq >= 0) AND ((status = ANY (ARRAY['DRAFT'::text, 'CANCELLED'::text])) OR (submission_seq >= 1)));
-- Create "kyc_registration_checks" table
CREATE TABLE "seller"."kyc_registration_checks" (
  "check_id" uuid NOT NULL,
  "application_id" uuid NOT NULL,
  "seq" bigint NOT NULL,
  "request_kind" text NOT NULL,
  "requested_by_admin_id" bigint NULL,
  "requested_at" timestamptz NOT NULL,
  "submission_seq" bigint NOT NULL,
  "outcome" text NOT NULL DEFAULT 'PENDING',
  "queried_at" timestamptz NULL,
  "tax_id" text NULL,
  "name" text NULL,
  "address_text" text NULL,
  "town_code" text NULL,
  "address_detail" text NULL,
  "setup_date" date NULL,
  "organization_type" text NULL,
  "status" text NULL,
  "status_text" text NULL,
  "suspended_from" date NULL,
  "suspended_until" date NULL,
  "mismatched_fields" text[] NOT NULL DEFAULT '{}',
  "unverifiable_fields" text[] NOT NULL DEFAULT '{}',
  PRIMARY KEY ("check_id"),
  CONSTRAINT "kyc_registration_checks_seq_key" UNIQUE ("application_id", "seq"),
  CONSTRAINT "kyc_registration_checks_application_id_fkey" FOREIGN KEY ("application_id") REFERENCES "seller"."kyc_applications" ("application_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "kyc_registration_checks_fields_check" CHECK ((mismatched_fields <@ ARRAY['business.company_name'::text, 'business.company_address'::text, 'business.company_register_date'::text, 'business.company_type'::text]) AND (unverifiable_fields <@ ARRAY['business.company_name'::text, 'business.company_address'::text, 'business.company_register_date'::text, 'business.company_type'::text]) AND (NOT (mismatched_fields && unverifiable_fields))),
  CONSTRAINT "kyc_registration_checks_found_check" CHECK (((outcome = 'FOUND'::text) AND (tax_id IS NOT NULL) AND (tax_id ~ '^[0-9]{8}$'::text) AND (name IS NOT NULL) AND (char_length(name) >= 1) AND (address_text IS NOT NULL) AND (status IS NOT NULL) AND (status = ANY (ARRAY['ACTIVE'::text, 'ORDERED_TO_DISSOLVE'::text, 'TERMINATED'::text, 'UNKNOWN'::text])) AND (status_text IS NOT NULL)) OR ((outcome <> 'FOUND'::text) AND (tax_id IS NULL) AND (name IS NULL) AND (address_text IS NULL) AND (town_code IS NULL) AND (address_detail IS NULL) AND (setup_date IS NULL) AND (organization_type IS NULL) AND (status IS NULL) AND (status_text IS NULL) AND (suspended_from IS NULL) AND (suspended_until IS NULL) AND (mismatched_fields = '{}'::text[]) AND (unverifiable_fields = '{}'::text[]))),
  CONSTRAINT "kyc_registration_checks_kind_check" CHECK (((request_kind = 'SUBMIT'::text) AND (requested_by_admin_id IS NULL)) OR ((request_kind = 'ADMIN_REQUERY'::text) AND (requested_by_admin_id IS NOT NULL) AND (requested_by_admin_id > 0))),
  CONSTRAINT "kyc_registration_checks_outcome_check" CHECK (outcome = ANY (ARRAY['PENDING'::text, 'FOUND'::text, 'NOT_FOUND'::text, 'UNAVAILABLE'::text, 'CANCELLED'::text])),
  CONSTRAINT "kyc_registration_checks_queried_check" CHECK ((outcome = 'PENDING'::text) = (queried_at IS NULL)),
  CONSTRAINT "kyc_registration_checks_seq_check" CHECK (seq >= 1),
  CONSTRAINT "kyc_registration_checks_submission_seq_check" CHECK (submission_seq >= 1),
  CONSTRAINT "kyc_registration_checks_suspension_check" CHECK ((suspended_from IS NULL) OR (suspended_until IS NULL) OR (suspended_until >= suspended_from)),
  CONSTRAINT "kyc_registration_checks_town_check" CHECK (((town_code IS NULL) = (address_detail IS NULL)) AND ((town_code IS NULL) OR (town_code ~ '^[0-9]{8}$'::text)))
);
-- Modify "kyc_reviews" table
ALTER TABLE "seller"."kyc_reviews" ADD CONSTRAINT "kyc_reviews_registration_check_check" CHECK ((registration_check_id IS NULL) OR (action = ANY (ARRAY['APPROVE'::text, 'REJECT'::text]))), ADD COLUMN "registration_check_id" uuid NULL, ADD CONSTRAINT "kyc_reviews_registration_check_id_fkey" FOREIGN KEY ("registration_check_id") REFERENCES "seller"."kyc_registration_checks" ("check_id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Create index "kyc_reviews_registration_check" to table: "kyc_reviews"
CREATE INDEX "kyc_reviews_registration_check" ON "seller"."kyc_reviews" ("registration_check_id") WHERE (registration_check_id IS NOT NULL);
