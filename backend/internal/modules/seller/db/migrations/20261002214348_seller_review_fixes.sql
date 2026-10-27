-- Modify "kyc_reviews" table
ALTER TABLE "seller"."kyc_reviews" DROP CONSTRAINT "kyc_reviews_action_check", ADD CONSTRAINT "kyc_reviews_action_check" CHECK (action = ANY (ARRAY['SUBMIT'::text, 'APPROVE'::text, 'REJECT'::text, 'CANCEL'::text, 'REBIND'::text])), DROP CONSTRAINT "kyc_reviews_reviewer_check", ADD CONSTRAINT "kyc_reviews_reviewer_check" CHECK (((action = ANY (ARRAY['APPROVE'::text, 'REJECT'::text])) AND (reviewer_admin_id IS NOT NULL) AND (reviewer_admin_id > 0) AND (reviewer_name IS NOT NULL)) OR ((action = 'REBIND'::text) AND (reviewer_admin_id IS NOT NULL) AND (reviewer_admin_id > 0)) OR ((action = ANY (ARRAY['SUBMIT'::text, 'CANCEL'::text])) AND (reviewer_admin_id IS NULL) AND (reviewer_name IS NULL)));
-- Drop index "payuni_registrations_poll" from table: "payuni_registrations"
DROP INDEX "seller"."payuni_registrations_poll";
-- Modify "payuni_registrations" table
ALTER TABLE "seller"."payuni_registrations" DROP COLUMN "claimed_until", ADD COLUMN "resubmitted_at" timestamptz NULL;
-- Create index "payuni_registrations_poll" to table: "payuni_registrations"
CREATE INDEX "payuni_registrations_poll" ON "seller"."payuni_registrations" ("status_checked_at" NULLS FIRST, "store_id") WHERE ((bind_status = 'BOUND'::text) AND ((outcome = ANY (ARRAY['NONE'::text, 'UNDER_REVIEW'::text, 'PENDING_INFO'::text, 'UNRECOGNIZED'::text])) OR (resubmitted_at IS NOT NULL)));
