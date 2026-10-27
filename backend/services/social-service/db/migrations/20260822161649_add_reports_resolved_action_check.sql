-- Modify "reports" table
ALTER TABLE "reports" ADD CONSTRAINT "chk_reports_resolved_action" CHECK ((resolved_action IS NULL) OR ((resolved_action)::text = ANY ((ARRAY['HIDE_TARGET'::character varying, 'WARN_USER'::character varying, 'BAN_USER'::character varying, 'DISMISS'::character varying])::text[])));
