-- Drop index "kyc_applications_created" from table: "kyc_applications"
DROP INDEX "seller"."kyc_applications_created";
-- Drop index "kyc_applications_status" from table: "kyc_applications"
DROP INDEX "seller"."kyc_applications_status";
-- Create index "kyc_applications_status_submitted" to table: "kyc_applications"
CREATE INDEX "kyc_applications_status_submitted" ON "seller"."kyc_applications" ("status", "submitted_at", "application_id") WHERE ((status <> 'DRAFT'::text) AND (submitted_at IS NOT NULL));
-- Create index "kyc_applications_submitted" to table: "kyc_applications"
CREATE INDEX "kyc_applications_submitted" ON "seller"."kyc_applications" ("submitted_at", "application_id") WHERE ((status <> 'DRAFT'::text) AND (submitted_at IS NOT NULL));
-- Modify "kyc_reviews" table
ALTER TABLE "seller"."kyc_reviews" DROP CONSTRAINT "kyc_reviews_reviewer_name_check", DROP CONSTRAINT "kyc_reviews_reviewer_check", ADD CONSTRAINT "kyc_reviews_reviewer_check" CHECK (((action = ANY (ARRAY['APPROVE'::text, 'REJECT'::text, 'REBIND'::text])) AND (reviewer_admin_id IS NOT NULL) AND (reviewer_admin_id > 0)) OR ((action = ANY (ARRAY['SUBMIT'::text, 'CANCEL'::text])) AND (reviewer_admin_id IS NULL))), DROP COLUMN "reviewer_name";
