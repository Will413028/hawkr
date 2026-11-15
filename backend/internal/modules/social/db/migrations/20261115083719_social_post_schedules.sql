-- Create "post_schedules" table
CREATE TABLE "social"."post_schedules" (
  "id" uuid NOT NULL,
  "post_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "publish_at" timestamptz NOT NULL,
  "scheduled_by_kind" text NOT NULL,
  "scheduled_by_ref" text NOT NULL,
  "state" text NOT NULL,
  "cancel_reason" text NULL,
  "created_at" timestamptz NOT NULL,
  "resolved_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "post_schedules_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "social"."posts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "post_schedules_by_kind_check" CHECK (scheduled_by_kind = ANY (ARRAY['OWNER'::text, 'DELEGATION'::text])),
  CONSTRAINT "post_schedules_by_ref_check" CHECK ((char_length(scheduled_by_ref) >= 1) AND (char_length(scheduled_by_ref) <= 200) AND (scheduled_by_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "post_schedules_reason_check" CHECK (cancel_reason = ANY (ARRAY['AUTHORITY_LOST'::text, 'DELETED'::text, 'TAKEN_DOWN'::text, 'REPLACED'::text, 'OWNER_CANCELED'::text, 'NOT_PUBLISHABLE'::text, 'RETRY_EXHAUSTED'::text, 'EXPIRED'::text])),
  CONSTRAINT "post_schedules_reason_state_check" CHECK ((state = 'CANCELED'::text) = (cancel_reason IS NOT NULL)),
  CONSTRAINT "post_schedules_resolved_check" CHECK ((state = 'PENDING'::text) = (resolved_at IS NULL)),
  CONSTRAINT "post_schedules_state_check" CHECK (state = ANY (ARRAY['PENDING'::text, 'PUBLISHED'::text, 'CANCELED'::text])),
  CONSTRAINT "post_schedules_store_check" CHECK (store_id > 0)
);
-- Create index "post_schedules_by_pending" to table: "post_schedules"
CREATE INDEX "post_schedules_by_pending" ON "social"."post_schedules" ("scheduled_by_kind", "scheduled_by_ref") WHERE (state = 'PENDING'::text);
-- Create index "post_schedules_post_pending" to table: "post_schedules"
CREATE UNIQUE INDEX "post_schedules_post_pending" ON "social"."post_schedules" ("post_id") WHERE (state = 'PENDING'::text);
-- Create index "post_schedules_store_pending" to table: "post_schedules"
CREATE INDEX "post_schedules_store_pending" ON "social"."post_schedules" ("store_id") WHERE (state = 'PENDING'::text);
-- Set comment to column: "scheduled_by_ref" on table: "post_schedules"
COMMENT ON COLUMN "social"."post_schedules"."scheduled_by_ref" IS '發起者擁有格式的不透明字串；DELEGATION 放代行 session 的 ID，OWNER 放店家帳號 ID 的十進位字串';
-- Set comment to column: "cancel_reason" on table: "post_schedules"
COMMENT ON COLUMN "social"."post_schedules"."cancel_reason" IS '取消原因，凍結在列上：AUTHORITY_LOST 到點時授權已失效、DELETED 貼文被刪、TAKEN_DOWN 貼文被下架措施、REPLACED 被立即發布或新排程取代、OWNER_CANCELED 店主取消、NOT_PUBLISHABLE 到點時貼文不再滿足公開要件、RETRY_EXHAUSTED 到點發布重試用盡、EXPIRED 到點時已超過可補發的期限';
