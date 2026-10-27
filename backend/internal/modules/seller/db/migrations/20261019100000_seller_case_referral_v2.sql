-- 轉報紀錄 v2（A3b 步驟 10e 第 3c-b-2 刀，rb R5）：加追蹤日期、「不轉報」與主管機關對轉報的回覆。
-- 既有列一律是已轉報（REFERRED），追蹤日期是 NULL（轉換不編日期）。
ALTER TABLE "seller"."case_referrals" RENAME COLUMN "referred_at" TO "recorded_at";
ALTER TABLE "seller"."case_referrals" RENAME COLUMN "admin_id" TO "recorded_by_admin_id";
-- 欄名改了，case_referrals_admin_check 的內文由 PostgreSQL 一起改。
-- NOT NULL 約束的名稱不會跟著欄名改，改成和 schema.sql 重新建出來的一致。
ALTER TABLE "seller"."case_referrals" RENAME CONSTRAINT "case_referrals_referred_at_not_null" TO "case_referrals_recorded_at_not_null";
ALTER TABLE "seller"."case_referrals" RENAME CONSTRAINT "case_referrals_admin_id_not_null" TO "case_referrals_recorded_by_admin_id_not_null";
ALTER TABLE "seller"."case_referrals"
  ADD COLUMN "outcome" text NOT NULL DEFAULT 'REFERRED',
  ALTER COLUMN "channel" DROP NOT NULL,
  ADD COLUMN "follow_up_on" date NULL,
  ADD COLUMN "waiver_reason" text NULL;
ALTER TABLE "seller"."case_referrals" ALTER COLUMN "outcome" DROP DEFAULT;
ALTER TABLE "seller"."case_referrals"
  ADD CONSTRAINT "case_referrals_outcome_check" CHECK (((((outcome = 'REFERRED'::text) AND (channel IS NOT NULL) AND (waiver_reason IS NULL)) OR ((outcome = 'WAIVED'::text) AND (channel IS NULL) AND (reference_no IS NULL) AND (follow_up_on IS NULL) AND ((char_length(waiver_reason) >= 1) AND (char_length(waiver_reason) <= 2000))))) IS TRUE),
  ADD CONSTRAINT "case_referrals_outcome_key" UNIQUE ("case_id", "outcome");
-- Create "case_referral_replies" table
CREATE TABLE "seller"."case_referral_replies" (
  "id" uuid NOT NULL,
  "case_id" uuid NOT NULL,
  "outcome" text NOT NULL DEFAULT 'REFERRED',
  "body" text NOT NULL,
  "received_on" date NOT NULL,
  "recorded_by_admin_id" bigint NOT NULL,
  "recorded_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "case_referral_replies_referral_fkey" FOREIGN KEY ("case_id", "outcome") REFERENCES "seller"."case_referrals" ("case_id", "outcome") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "case_referral_replies_admin_check" CHECK (recorded_by_admin_id > 0),
  CONSTRAINT "case_referral_replies_body_check" CHECK ((char_length(body) >= 1) AND (char_length(body) <= 2000)),
  CONSTRAINT "case_referral_replies_outcome_check" CHECK (outcome = 'REFERRED'::text)
);
-- Create index "case_referral_replies_case" to table: "case_referral_replies"
CREATE INDEX "case_referral_replies_case" ON "seller"."case_referral_replies" ("case_id", "recorded_at", "id");
