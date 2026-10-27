-- Create "food_safety_reports" table
CREATE TABLE "food_safety_reports" (
  "id" bigint NOT NULL,
  "target_seller_id" bigint NULL,
  "target_product_id" bigint NULL,
  "target_order_no" character varying(50) NULL,
  "reporter_phone" character varying(20) NOT NULL,
  "reporter_email" character varying(255) NULL,
  "reporter_name" character varying(100) NULL,
  "description" text NOT NULL,
  "evidence_asset_ids" jsonb NULL,
  "status" character varying(20) NOT NULL DEFAULT 'pending',
  "acknowledged_at" timestamptz NULL,
  "acknowledged_by_admin_id" bigint NULL,
  "investigating_at" timestamptz NULL,
  "resolved_at" timestamptz NULL,
  "admin_resolution_summary" text NULL,
  "fda_reported_at" timestamptz NULL,
  "fda_report_channel" character varying(30) NULL,
  "fda_reference_no" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "idx_fsr_acknowledged_at" to table: "food_safety_reports"
CREATE INDEX "idx_fsr_acknowledged_at" ON "food_safety_reports" ("acknowledged_at") WHERE (acknowledged_at IS NULL);
-- Create index "idx_fsr_phone_seller_created" to table: "food_safety_reports"
CREATE INDEX "idx_fsr_phone_seller_created" ON "food_safety_reports" ("reporter_phone", "target_seller_id", "created_at" DESC);
-- Create index "idx_fsr_status_created" to table: "food_safety_reports"
CREATE INDEX "idx_fsr_status_created" ON "food_safety_reports" ("status", "created_at");
-- Create index "uq_food_safety_reports_phone_seller_week" to table: "food_safety_reports"
CREATE UNIQUE INDEX "uq_food_safety_reports_phone_seller_week" ON "food_safety_reports" ("reporter_phone", "target_seller_id", (date_trunc('week'::text, (created_at AT TIME ZONE 'UTC'::text))));
-- Create "seller_notification_log" table
CREATE TABLE "seller_notification_log" (
  "id" bigint NOT NULL,
  "source_report_id" bigint NULL,
  "source_report_table" character varying(30) NULL,
  "template_key" character varying(50) NOT NULL,
  "seller_id" bigint NOT NULL,
  "sent_by_admin_id" bigint NULL,
  "sent_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "message_payload" jsonb NOT NULL,
  PRIMARY KEY ("id")
);
-- Create index "idx_snl_seller_sent" to table: "seller_notification_log"
CREATE INDEX "idx_snl_seller_sent" ON "seller_notification_log" ("seller_id", "sent_at" DESC);
-- Create index "idx_snl_source" to table: "seller_notification_log"
CREATE INDEX "idx_snl_source" ON "seller_notification_log" ("source_report_table", "source_report_id") WHERE (source_report_id IS NOT NULL);
-- Create "seller_violation_reports" table
CREATE TABLE "seller_violation_reports" (
  "id" bigint NOT NULL,
  "target_seller_id" bigint NOT NULL,
  "target_product_id" bigint NULL,
  "target_post_id" bigint NULL,
  "target_order_no" character varying(50) NULL,
  "reporter_account_id" bigint NOT NULL,
  "category" character varying(30) NOT NULL,
  "severity" character varying(10) NOT NULL,
  "description" text NOT NULL,
  "evidence_asset_ids" jsonb NULL,
  "status" character varying(20) NOT NULL DEFAULT 'pending',
  "investigating_at" timestamptz NULL,
  "resolved_at" timestamptz NULL,
  "admin_resolution_summary" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "idx_svr_reporter_created" to table: "seller_violation_reports"
CREATE INDEX "idx_svr_reporter_created" ON "seller_violation_reports" ("reporter_account_id", "created_at" DESC);
-- Create index "idx_svr_status_severity_created" to table: "seller_violation_reports"
CREATE INDEX "idx_svr_status_severity_created" ON "seller_violation_reports" ("status", "severity", "created_at");
-- Create index "idx_svr_target_seller_status" to table: "seller_violation_reports"
CREATE INDEX "idx_svr_target_seller_status" ON "seller_violation_reports" ("target_seller_id", "status");
-- Create index "uq_seller_violation_reports_dedup_calendar_day" to table: "seller_violation_reports"
CREATE UNIQUE INDEX "uq_seller_violation_reports_dedup_calendar_day" ON "seller_violation_reports" ("reporter_account_id", "target_seller_id", "category", (((created_at AT TIME ZONE 'UTC'::text))::date));
-- Create "seller_violations" table
CREATE TABLE "seller_violations" (
  "id" bigint NOT NULL,
  "seller_id" bigint NOT NULL,
  "source_seller_report_id" bigint NULL,
  "source_food_safety_report_id" bigint NULL,
  "category" character varying(30) NOT NULL,
  "severity" character varying(10) NOT NULL,
  "penalty_type" character varying(30) NOT NULL,
  "penalty_amount_cents" bigint NULL,
  "description" text NOT NULL,
  "public_summary" text NULL,
  "effective_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "expires_at" timestamptz NULL,
  "status" character varying(20) NOT NULL DEFAULT 'enforced',
  "revoked_at" timestamptz NULL,
  "revoked_reason" character varying(50) NULL,
  "appealed" boolean NOT NULL DEFAULT false,
  "appeal_reason" text NULL,
  "appeal_evidence_asset_ids" jsonb NULL,
  "appeal_status" character varying(20) NULL,
  "appeal_submitted_at" timestamptz NULL,
  "appeal_decided_at" timestamptz NULL,
  "appeal_decision_reason" text NULL,
  "appeal_decided_by_admin_id" bigint NULL,
  "created_by_admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "seller_violations_source_food_safety_report_id_fkey" FOREIGN KEY ("source_food_safety_report_id") REFERENCES "food_safety_reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "seller_violations_source_seller_report_id_fkey" FOREIGN KEY ("source_seller_report_id") REFERENCES "seller_violation_reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "chk_sv_penalty_fields" CHECK ((((penalty_type)::text = 'warning'::text) AND (penalty_amount_cents IS NULL) AND (expires_at IS NULL)) OR (((penalty_type)::text = 'full_suspension'::text) AND (penalty_amount_cents IS NULL) AND (expires_at IS NOT NULL)) OR (((penalty_type)::text = 'fine'::text) AND (penalty_amount_cents IS NOT NULL) AND (expires_at IS NULL)) OR (((penalty_type)::text = 'partial_suspension'::text) AND (penalty_amount_cents IS NULL) AND (expires_at IS NOT NULL)) OR ((penalty_type)::text = ANY ((ARRAY['permanent_ban'::character varying, 'mandatory_training'::character varying])::text[]))),
  CONSTRAINT "chk_sv_source_mutex" CHECK ((((source_seller_report_id IS NOT NULL))::integer + ((source_food_safety_report_id IS NOT NULL))::integer) <= 1),
  CONSTRAINT "chk_sv_status" CHECK ((status)::text = ANY ((ARRAY['enforced'::character varying, 'revoked'::character varying])::text[]))
);
-- Create index "idx_sv_appeal_pending" to table: "seller_violations"
CREATE INDEX "idx_sv_appeal_pending" ON "seller_violations" ("appeal_submitted_at") WHERE ((appeal_status)::text = 'pending'::text);
-- Create index "idx_sv_expires_at_full_suspension" to table: "seller_violations"
CREATE INDEX "idx_sv_expires_at_full_suspension" ON "seller_violations" ("expires_at") WHERE (((penalty_type)::text = 'full_suspension'::text) AND (revoked_at IS NULL));
-- Create index "idx_sv_seller_created" to table: "seller_violations"
CREATE INDEX "idx_sv_seller_created" ON "seller_violations" ("seller_id", "created_at" DESC);
-- Create index "idx_sv_seller_severity_effective" to table: "seller_violations"
CREATE INDEX "idx_sv_seller_severity_effective" ON "seller_violations" ("seller_id", "severity", "effective_at" DESC);
-- Create index "idx_sv_seller_severity_revoked" to table: "seller_violations"
CREATE INDEX "idx_sv_seller_severity_revoked" ON "seller_violations" ("seller_id", "severity", "revoked_at") WHERE (revoked_at IS NULL);
