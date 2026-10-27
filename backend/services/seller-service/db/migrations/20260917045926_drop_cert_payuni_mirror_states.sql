UPDATE "certification_applications"
SET status = 'forwarded',
    forwarded_at = COALESCE(forwarded_at, approved_at, submitted_at, updated_at)
WHERE status IN ('approved', 'pending_info');
ALTER TABLE "certification_applications" DROP CONSTRAINT "certification_applications_status_check", ADD CONSTRAINT "certification_applications_status_check" CHECK ((status)::text = ANY ((ARRAY['draft'::character varying, 'submitted'::character varying, 'forwarded'::character varying, 'forward_failed'::character varying, 'rejected'::character varying, 'cancelled'::character varying])::text[]));
