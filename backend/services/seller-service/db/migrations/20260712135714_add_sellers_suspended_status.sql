-- Modify "sellers" table
ALTER TABLE "sellers" DROP CONSTRAINT "chk_sellers_status", ADD CONSTRAINT "chk_sellers_status" CHECK ((status)::text = ANY ((ARRAY['active'::character varying, 'suspended'::character varying, 'suspended_governance'::character varying, 'closed'::character varying])::text[]));
