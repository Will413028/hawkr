-- Modify "post_schedules" table
ALTER TABLE "social"."post_schedules" DROP CONSTRAINT "post_schedules_reason_check", ADD CONSTRAINT "post_schedules_reason_check" CHECK (cancel_reason = ANY (ARRAY['AUTHORITY_LOST'::text, 'DELETED'::text, 'TAKEN_DOWN'::text, 'REPLACED'::text, 'OWNER_CANCELED'::text, 'NOT_PUBLISHABLE'::text]));
-- Set comment to column: "cancel_reason" on table: "post_schedules"
COMMENT ON COLUMN "social"."post_schedules"."cancel_reason" IS '取消原因，凍結在列上：AUTHORITY_LOST 到點時授權已失效、DELETED 貼文被刪、TAKEN_DOWN 貼文被下架措施、REPLACED 被立即發布或新排程取代、OWNER_CANCELED 店主取消、NOT_PUBLISHABLE 到點時貼文不再滿足公開要件';
