-- Drop index "idx_moderation_actions_user" from table: "moderation_actions"
DROP INDEX "idx_moderation_actions_user";
-- Modify "moderation_actions" table
ALTER TABLE "moderation_actions" DROP COLUMN "user_id", ADD COLUMN "affected_user_id" bigint NULL, ADD COLUMN "previous_state" jsonb NULL, ADD COLUMN "new_state" jsonb NULL;
-- Create index "idx_moderation_actions_user" to table: "moderation_actions"
CREATE INDEX "idx_moderation_actions_user" ON "moderation_actions" ("affected_user_id", "create_time" DESC);
-- Modify "reports" table
ALTER TABLE "reports" ADD CONSTRAINT "chk_reports_reason_code" CHECK ((reason_code)::text = ANY ((ARRAY['SPAM'::character varying, 'SEXUAL'::character varying, 'FRAUD'::character varying, 'HARASSMENT'::character varying, 'FOOD_SAFETY'::character varying, 'POLITICAL'::character varying, 'FAKE_NEWS'::character varying, 'OTHER'::character varying])::text[])), ADD CONSTRAINT "chk_reports_status" CHECK ((status)::text = ANY ((ARRAY['PENDING'::character varying, 'RESOLVED_HIDDEN'::character varying, 'RESOLVED_WARNED'::character varying, 'RESOLVED_BANNED'::character varying, 'DISMISSED'::character varying, 'APPEALED'::character varying])::text[])), ADD COLUMN "target_owner_id" bigint NULL;
