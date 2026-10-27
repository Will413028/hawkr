-- 案件的詢問（A3b 步驟 10e 第 3c-b 刀，rb R4）：平台對案件的賣家的一次要求，有期限、一則回覆、可撤回，每個案件同時最多一筆還沒結束的；
-- 回覆的附件是機敏檔，讀取紀錄的擁有者加詢問。舊的配合要求表在下一支 migration 刪除。
-- Create "case_inquiries" table
CREATE TABLE "seller"."case_inquiries" (
  "id" uuid NOT NULL,
  "case_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "kind" text NOT NULL,
  "message" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  "created_by_admin_id" bigint NOT NULL,
  "due_at" timestamptz NOT NULL,
  "request_id" uuid NULL,
  "request_hash" bytea NULL,
  "reply_body" text NULL,
  "reply_channel" text NULL,
  "reply_received_at" timestamptz NULL,
  "reply_recorded_at" timestamptz NULL,
  "reply_recorded_by_admin_id" bigint NULL,
  "withdrawn_at" timestamptz NULL,
  "withdrawn_by_admin_id" bigint NULL,
  "withdraw_cause" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "case_inquiries_request_key" UNIQUE ("created_by_admin_id", "request_id"),
  CONSTRAINT "case_inquiries_case_fkey" FOREIGN KEY ("case_id", "store_id") REFERENCES "seller"."cases" ("id", "store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "case_inquiries_admin_check" CHECK (created_by_admin_id > 0),
  CONSTRAINT "case_inquiries_due_check" CHECK (due_at > created_at),
  CONSTRAINT "case_inquiries_kind_check" CHECK (kind = ANY (ARRAY['RECORDS'::text, 'EXPLANATION'::text])),
  CONSTRAINT "case_inquiries_message_check" CHECK ((char_length(message) >= 1) AND (char_length(message) <= 2000)),
  CONSTRAINT "case_inquiries_reply_check" CHECK ((((reply_body IS NULL) AND (reply_channel IS NULL) AND (reply_received_at IS NULL) AND (reply_recorded_at IS NULL) AND (reply_recorded_by_admin_id IS NULL)) OR (((char_length(reply_body) >= 1) AND (char_length(reply_body) <= 4000)) AND (reply_channel = ANY (ARRAY['PHONE'::text, 'EMAIL'::text, 'MESSAGING'::text, 'OTHER'::text])) AND (reply_received_at >= created_at) AND (reply_recorded_at >= reply_received_at) AND (reply_recorded_by_admin_id > 0))) IS TRUE),
  CONSTRAINT "case_inquiries_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)),
  CONSTRAINT "case_inquiries_terminal_check" CHECK ((reply_received_at IS NULL) OR (withdrawn_at IS NULL)),
  CONSTRAINT "case_inquiries_withdraw_check" CHECK ((((withdrawn_at IS NULL) AND (withdrawn_by_admin_id IS NULL) AND (withdraw_cause IS NULL)) OR ((withdrawn_at >= created_at) AND (withdrawn_by_admin_id > 0) AND (withdraw_cause = ANY (ARRAY['ADMIN'::text, 'CASE_CLOSED'::text])))) IS TRUE)
);
-- Create index "case_inquiries_case" to table: "case_inquiries"
CREATE INDEX "case_inquiries_case" ON "seller"."case_inquiries" ("case_id", "created_at", "id");
-- Create index "case_inquiries_one_open" to table: "case_inquiries"
CREATE UNIQUE INDEX "case_inquiries_one_open" ON "seller"."case_inquiries" ("case_id") WHERE ((reply_received_at IS NULL) AND (withdrawn_at IS NULL));
-- Create index "case_inquiries_open_due" to table: "case_inquiries"
CREATE INDEX "case_inquiries_open_due" ON "seller"."case_inquiries" ("due_at", "id") WHERE ((reply_received_at IS NULL) AND (withdrawn_at IS NULL));
-- Create "case_inquiry_reply_evidence" table
CREATE TABLE "seller"."case_inquiry_reply_evidence" (
  "inquiry_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "asset_id" text NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("inquiry_id", "position"),
  CONSTRAINT "case_inquiry_reply_evidence_inquiry_id_fkey" FOREIGN KEY ("inquiry_id") REFERENCES "seller"."case_inquiries" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "case_inquiry_reply_evidence_asset_check" CHECK ((asset_id <> ''::text) AND (object_key <> ''::text)),
  CONSTRAINT "case_inquiry_reply_evidence_position_check" CHECK (("position" >= 0) AND ("position" <= 4))
);
-- Modify "evidence_accesses" table
ALTER TABLE "seller"."evidence_accesses" DROP CONSTRAINT "evidence_accesses_owner_check", ADD CONSTRAINT "evidence_accesses_owner_check" CHECK (num_nonnulls(report_id, request_id, inquiry_id) = 1), ADD COLUMN "inquiry_id" uuid NULL, ADD CONSTRAINT "evidence_accesses_inquiry_fkey" FOREIGN KEY ("inquiry_id", "position") REFERENCES "seller"."case_inquiry_reply_evidence" ("inquiry_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "evidence_accesses_inquiry" to table: "evidence_accesses"
CREATE INDEX "evidence_accesses_inquiry" ON "seller"."evidence_accesses" ("inquiry_id", "accessed_at" DESC) WHERE (inquiry_id IS NOT NULL);
