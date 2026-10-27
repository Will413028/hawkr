-- Modify "payuni_registrations" table
ALTER TABLE "payuni_registrations" ADD COLUMN "reason" text NULL;
-- Create "payuni_status_history" table
CREATE TABLE "payuni_status_history" (
  "id" bigint NOT NULL,
  "account_id" bigint NOT NULL,
  "payuni_mer_id" character varying(50) NOT NULL,
  "source" character varying(20) NOT NULL,
  "mer_status" character varying(10) NULL,
  "member_status" character varying(10) NULL,
  "credit_status" character varying(10) NULL,
  "applepay_status" character varying(10) NULL,
  "googlepay_status" character varying(10) NULL,
  "atm_status" character varying(10) NULL,
  "reason" text NULL,
  "raw_response" jsonb NULL,
  "recorded_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "idx_payuni_status_history_account_recorded" to table: "payuni_status_history"
CREATE INDEX "idx_payuni_status_history_account_recorded" ON "payuni_status_history" ("account_id", "recorded_at" DESC);
