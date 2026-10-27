-- Modify "vendor_portal_sessions" table
ALTER TABLE "vendor_portal_sessions" DROP CONSTRAINT "chk_vps_assurance", ADD CONSTRAINT "chk_vps_assurance" CHECK ((assurance)::text = ANY ((ARRAY['INVITE_ONLY'::character varying, 'GOOGLE_VERIFIED'::character varying])::text[]));
