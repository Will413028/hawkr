-- Backfill: retire legacy pre-admin-gate states → submitted (Tier 1.5).
-- Old semantics: under_review = PayUni already approved the merchant, ADMIN review
-- still pending (old payuni_approve: {submitted,pending_info}→under_review); legacy
-- pending_info was likewise reached from submitted BEFORE any admin review. Under
-- the new FSM pending_info sits AFTER the gate (payuni_approve: pending_info→approved),
-- so leaving legacy rows in place would let the poller auto-approve them without an
-- admin ever acting. Mapping to forwarded/forward_failed would equally skip the gate
-- and dead-end (poller only reads 'pending' registrations; Approve rejects those
-- states as duplicates). 'submitted' re-enters the admin queue; on approve, the
-- relink path funnels PayUni state (active→approved, member-KYC hold→pending_info)
-- through the same outcome path as the poller, so nothing is lost.
-- Must run BEFORE the status CHECK constraint below, which forbids 'under_review'
-- (Postgres validates existing rows on ADD CONSTRAINT).
UPDATE certification_applications SET status = 'submitted' WHERE status IN ('under_review', 'pending_info');
-- Backfill: normalize legacy seller_type values before the seller_type CHECK below.
-- dev has a pre-vocabulary row with seller_type = 'personal' (cancelled application);
-- the current vocabulary is individual/business ('personal' = 自然人 = individual).
-- Catch-all NOT IN guards against any other pre-vocabulary stragglers; NULL passes
-- the CHECK on its own and is left untouched.
UPDATE certification_applications SET seller_type = 'individual'
WHERE seller_type IS NOT NULL AND seller_type NOT IN ('individual', 'business');
-- Modify "certification_applications" table
ALTER TABLE "certification_applications" ADD CONSTRAINT "certification_applications_seller_type_check" CHECK ((seller_type)::text = ANY ((ARRAY['individual'::character varying, 'business'::character varying])::text[])), ADD CONSTRAINT "certification_applications_status_check" CHECK ((status)::text = ANY ((ARRAY['draft'::character varying, 'submitted'::character varying, 'forwarded'::character varying, 'forward_failed'::character varying, 'pending_info'::character varying, 'approved'::character varying, 'rejected'::character varying, 'cancelled'::character varying])::text[])), ADD COLUMN "version" integer NOT NULL DEFAULT 0, ADD COLUMN "forwarded_at" timestamptz NULL;
-- Create index "idx_cert_app_account_created" to table: "certification_applications"
CREATE INDEX "idx_cert_app_account_created" ON "certification_applications" ("account_id", "created_at" DESC);
-- Create index "idx_cert_app_status" to table: "certification_applications"
CREATE INDEX "idx_cert_app_status" ON "certification_applications" ("status");
-- Modify "sellers" table
ALTER TABLE "sellers" DROP COLUMN "payuni_status", DROP COLUMN "approval_status", DROP COLUMN "overall_certification_status";
-- Create index "idx_sellers_current_certification_id" to table: "sellers"
CREATE INDEX "idx_sellers_current_certification_id" ON "sellers" ("current_certification_id");
