-- Modify "stores" table
ALTER TABLE "seller"."stores" DROP CONSTRAINT "stores_suspension_check", DROP COLUMN "suspension";
-- Create "reports" table
CREATE TABLE "seller"."reports" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "product_id" text NULL,
  "order_no" text NULL,
  "category" text NOT NULL,
  "description" text NOT NULL,
  "reporter_buyer_id" bigint NULL,
  "reporter_phone" text NULL,
  "reporter_email" text NULL,
  "reporter_name" text NULL,
  "contact_erased_at" timestamptz NULL,
  "status" text NOT NULL DEFAULT 'PENDING',
  "severity" text NULL,
  "acknowledged_at" timestamptz NULL,
  "acknowledged_by_admin_id" bigint NULL,
  "investigating_at" timestamptz NULL,
  "resolved_at" timestamptz NULL,
  "resolved_by_admin_id" bigint NULL,
  "resolution_note" text NULL,
  "fda_reported_at" timestamptz NULL,
  "fda_channel" text NULL,
  "fda_reference_no" text NULL,
  "fda_reported_by_admin_id" bigint NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "reports_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "reports_acknowledged_check" CHECK (((acknowledged_at IS NULL) = (acknowledged_by_admin_id IS NULL)) AND ((acknowledged_at IS NULL) = (severity IS NULL))),
  CONSTRAINT "reports_admin_check" CHECK ((acknowledged_by_admin_id > 0) AND (resolved_by_admin_id > 0) AND (fda_reported_by_admin_id > 0)),
  CONSTRAINT "reports_category_check" CHECK (category = ANY (ARRAY['FOOD_SAFETY'::text, 'FAKE_CERTIFICATION'::text, 'MISLEADING_INFO'::text, 'COUNTERFEIT'::text, 'UNAUTHORIZED_SALE'::text, 'FRAUD'::text, 'OTHER'::text])),
  CONSTRAINT "reports_description_check" CHECK ((char_length(description) >= 1) AND (char_length(description) <= 2000)),
  CONSTRAINT "reports_fda_channel_check" CHECK (fda_channel = ANY (ARRAY['HOTLINE_1919'::text, 'ONLINE_FORM'::text, 'FAX_HEALTH_DEPT'::text, 'IN_PERSON'::text, 'OTHER'::text])),
  CONSTRAINT "reports_fda_check" CHECK (((fda_reported_at IS NULL) AND (fda_channel IS NULL) AND (fda_reference_no IS NULL) AND (fda_reported_by_admin_id IS NULL)) OR ((category = 'FOOD_SAFETY'::text) AND (fda_reported_at IS NOT NULL) AND (fda_channel IS NOT NULL) AND (fda_reference_no IS NOT NULL) AND ((char_length(fda_reference_no) >= 1) AND (char_length(fda_reference_no) <= 100)) AND (fda_reported_by_admin_id IS NOT NULL))),
  CONSTRAINT "reports_order_no_check" CHECK ((char_length(order_no) >= 1) AND (char_length(order_no) <= 64)),
  CONSTRAINT "reports_product_id_check" CHECK ((char_length(product_id) >= 1) AND (char_length(product_id) <= 64)),
  CONSTRAINT "reports_reporter_check" CHECK (((reporter_buyer_id IS NOT NULL) AND (reporter_buyer_id > 0) AND (reporter_phone IS NULL) AND (reporter_email IS NULL) AND (reporter_name IS NULL) AND (contact_erased_at IS NULL)) OR ((reporter_buyer_id IS NULL) AND (contact_erased_at IS NULL) AND ((reporter_phone IS NOT NULL) OR (reporter_email IS NOT NULL))) OR ((reporter_buyer_id IS NULL) AND (contact_erased_at IS NOT NULL) AND (reporter_phone IS NULL) AND (reporter_email IS NULL) AND (reporter_name IS NULL))),
  CONSTRAINT "reports_reporter_email_check" CHECK ((reporter_email = lower(reporter_email)) AND (char_length(reporter_email) <= 254) AND (reporter_email ~ '^[^@[:space:]]+@[^@[:space:]]+$'::text)),
  CONSTRAINT "reports_reporter_name_check" CHECK ((char_length(reporter_name) >= 1) AND (char_length(reporter_name) <= 100)),
  CONSTRAINT "reports_reporter_phone_check" CHECK (reporter_phone ~ '^0[0-9]{8,9}$'::text),
  CONSTRAINT "reports_resolved_check" CHECK (((resolved_at IS NULL) = (resolved_by_admin_id IS NULL)) AND ((resolution_note IS NULL) OR ((resolved_at IS NOT NULL) AND ((char_length(resolution_note) >= 1) AND (char_length(resolution_note) <= 2000))))),
  CONSTRAINT "reports_severity_check" CHECK (severity = ANY (ARRAY['LOW'::text, 'MEDIUM'::text, 'HIGH'::text, 'CRITICAL'::text])),
  CONSTRAINT "reports_status_check" CHECK (status = ANY (ARRAY['PENDING'::text, 'INVESTIGATING'::text, 'VERIFIED'::text, 'DISMISSED'::text])),
  CONSTRAINT "reports_status_times_check" CHECK (((status = 'PENDING'::text) AND (investigating_at IS NULL) AND (resolved_at IS NULL)) OR ((status = 'INVESTIGATING'::text) AND (acknowledged_at IS NOT NULL) AND (investigating_at IS NOT NULL) AND (resolved_at IS NULL)) OR ((status = 'VERIFIED'::text) AND (acknowledged_at IS NOT NULL) AND (investigating_at IS NOT NULL) AND (resolved_at IS NOT NULL)) OR ((status = 'DISMISSED'::text) AND (resolved_at IS NOT NULL)))
);
-- Create index "reports_buyer_daily" to table: "reports"
CREATE UNIQUE INDEX "reports_buyer_daily" ON "seller"."reports" ("reporter_buyer_id", "store_id", "category", (((created_at AT TIME ZONE 'Asia/Taipei'::text))::date)) WHERE (reporter_buyer_id IS NOT NULL);
-- Create index "reports_contact_retention" to table: "reports"
CREATE INDEX "reports_contact_retention" ON "seller"."reports" ("created_at") WHERE ((reporter_buyer_id IS NULL) AND (contact_erased_at IS NULL));
-- Create index "reports_created" to table: "reports"
CREATE INDEX "reports_created" ON "seller"."reports" ("created_at" DESC, "id" DESC);
-- Create index "reports_email_weekly" to table: "reports"
CREATE UNIQUE INDEX "reports_email_weekly" ON "seller"."reports" ("reporter_email", "store_id", (date_trunc('week'::text, (created_at AT TIME ZONE 'Asia/Taipei'::text)))) WHERE (reporter_email IS NOT NULL);
-- Create index "reports_phone_weekly" to table: "reports"
CREATE UNIQUE INDEX "reports_phone_weekly" ON "seller"."reports" ("reporter_phone", "store_id", (date_trunc('week'::text, (created_at AT TIME ZONE 'Asia/Taipei'::text)))) WHERE (reporter_phone IS NOT NULL);
-- Create index "reports_reporter" to table: "reports"
CREATE INDEX "reports_reporter" ON "seller"."reports" ("reporter_buyer_id", "created_at" DESC, "id" DESC) WHERE (reporter_buyer_id IS NOT NULL);
-- Create index "reports_status" to table: "reports"
CREATE INDEX "reports_status" ON "seller"."reports" ("status", "created_at" DESC, "id" DESC);
-- Create index "reports_store" to table: "reports"
CREATE INDEX "reports_store" ON "seller"."reports" ("store_id", "created_at" DESC, "id" DESC);
-- Create "violations" table
CREATE TABLE "seller"."violations" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "source_report_id" uuid NULL,
  "category" text NOT NULL,
  "severity" text NOT NULL,
  "escalated_from" text NULL,
  "penalty" text NOT NULL,
  "description" text NOT NULL,
  "statement_to_seller" text NOT NULL,
  "status" text NOT NULL DEFAULT 'ENFORCED',
  "version" bigint NOT NULL DEFAULT 1,
  "created_by_admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "revoked_reason" text NULL,
  "revoked_by_admin_id" bigint NULL,
  "revoked_at" timestamptz NULL,
  "appeal_status" text NULL,
  "appeal_reason" text NULL,
  "appeal_submitted_at" timestamptz NULL,
  "appeal_decision_reason" text NULL,
  "appeal_decided_by_admin_id" bigint NULL,
  "appeal_decided_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "violations_source_report_id_fkey" FOREIGN KEY ("source_report_id") REFERENCES "seller"."reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "violations_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "violations_admin_check" CHECK ((created_by_admin_id > 0) AND (revoked_by_admin_id > 0)),
  CONSTRAINT "violations_appeal_check" CHECK (((appeal_status IS NULL) AND (appeal_reason IS NULL) AND (appeal_submitted_at IS NULL) AND (appeal_decision_reason IS NULL) AND (appeal_decided_by_admin_id IS NULL) AND (appeal_decided_at IS NULL)) OR ((appeal_status IS NOT NULL) AND (appeal_reason IS NOT NULL) AND ((char_length(appeal_reason) >= 1) AND (char_length(appeal_reason) <= 2000)) AND (appeal_submitted_at IS NOT NULL) AND (((appeal_status = 'PENDING'::text) AND (appeal_decision_reason IS NULL) AND (appeal_decided_by_admin_id IS NULL) AND (appeal_decided_at IS NULL)) OR ((appeal_status = ANY (ARRAY['ACCEPTED'::text, 'REJECTED'::text])) AND (appeal_decided_by_admin_id IS NOT NULL) AND (appeal_decided_by_admin_id > 0) AND (appeal_decided_at IS NOT NULL) AND (appeal_decided_at >= appeal_submitted_at) AND ((appeal_decision_reason IS NULL) OR ((char_length(appeal_decision_reason) >= 1) AND (char_length(appeal_decision_reason) <= 2000))) AND ((appeal_status = 'ACCEPTED'::text) OR (appeal_decision_reason IS NOT NULL)))))),
  CONSTRAINT "violations_appeal_revoked_check" CHECK (((NOT (appeal_status IS DISTINCT FROM 'ACCEPTED'::text)) = (NOT (revoked_reason IS DISTINCT FROM 'APPEAL_ACCEPTED'::text))) AND (NOT ((status = 'REVOKED'::text) AND (NOT (appeal_status IS DISTINCT FROM 'PENDING'::text))))),
  CONSTRAINT "violations_appeal_status_check" CHECK (appeal_status = ANY (ARRAY['PENDING'::text, 'ACCEPTED'::text, 'REJECTED'::text])),
  CONSTRAINT "violations_category_check" CHECK (category = ANY (ARRAY['FOOD_SAFETY'::text, 'FAKE_CERTIFICATION'::text, 'MISLEADING_INFO'::text, 'COUNTERFEIT'::text, 'UNAUTHORIZED_SALE'::text, 'FRAUD'::text, 'OTHER'::text])),
  CONSTRAINT "violations_description_check" CHECK ((char_length(description) >= 1) AND (char_length(description) <= 2000)),
  CONSTRAINT "violations_escalation_check" CHECK ((escalated_from IS NULL) OR ((escalated_from = 'MEDIUM'::text) AND (severity = 'HIGH'::text))),
  CONSTRAINT "violations_penalty_check" CHECK (penalty = ANY (ARRAY['WARNING'::text, 'SUSPENSION'::text])),
  CONSTRAINT "violations_revoked_check" CHECK (((status = 'REVOKED'::text) = (revoked_at IS NOT NULL)) AND ((revoked_at IS NULL) = (revoked_reason IS NULL)) AND ((revoked_at IS NULL) = (revoked_by_admin_id IS NULL)) AND ((revoked_at IS NULL) OR (revoked_at >= created_at))),
  CONSTRAINT "violations_revoked_reason_check" CHECK (revoked_reason = ANY (ARRAY['APPEAL_ACCEPTED'::text, 'ADMIN_CORRECTION'::text, 'EXTERNAL_DECISION'::text, 'OTHER'::text])),
  CONSTRAINT "violations_severity_check" CHECK (severity = ANY (ARRAY['LOW'::text, 'MEDIUM'::text, 'HIGH'::text, 'CRITICAL'::text])),
  CONSTRAINT "violations_statement_check" CHECK ((char_length(statement_to_seller) >= 1) AND (char_length(statement_to_seller) <= 2000)),
  CONSTRAINT "violations_status_check" CHECK (status = ANY (ARRAY['ENFORCED'::text, 'REVOKED'::text])),
  CONSTRAINT "violations_version_check" CHECK (version >= 1)
);
-- Create index "violations_created" to table: "violations"
CREATE INDEX "violations_created" ON "seller"."violations" ("created_at" DESC, "id" DESC);
-- Create index "violations_escalation" to table: "violations"
CREATE INDEX "violations_escalation" ON "seller"."violations" ("store_id", "created_at") WHERE ((status = 'ENFORCED'::text) AND (COALESCE(escalated_from, severity) = 'MEDIUM'::text));
-- Create index "violations_source_report" to table: "violations"
CREATE INDEX "violations_source_report" ON "seller"."violations" ("source_report_id") WHERE (source_report_id IS NOT NULL);
-- Create index "violations_store" to table: "violations"
CREATE INDEX "violations_store" ON "seller"."violations" ("store_id", "created_at" DESC, "id" DESC);
-- Create "appeal_evidence" table
CREATE TABLE "seller"."appeal_evidence" (
  "violation_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "asset_id" text NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("violation_id", "position"),
  CONSTRAINT "appeal_evidence_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "appeal_evidence_asset_check" CHECK ((asset_id <> ''::text) AND (object_key <> ''::text)),
  CONSTRAINT "appeal_evidence_position_check" CHECK (("position" >= 0) AND ("position" < 5))
);
-- Create "report_cooperation_requests" table
CREATE TABLE "seller"."report_cooperation_requests" (
  "report_id" uuid NOT NULL,
  "template" text NOT NULL,
  "requested_by_admin_id" bigint NOT NULL,
  "requested_at" timestamptz NOT NULL,
  PRIMARY KEY ("report_id", "template"),
  CONSTRAINT "report_cooperation_requests_report_id_fkey" FOREIGN KEY ("report_id") REFERENCES "seller"."reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_cooperation_requests_admin_check" CHECK (requested_by_admin_id > 0),
  CONSTRAINT "report_cooperation_requests_template_check" CHECK (template = ANY (ARRAY['ASSIST_INVESTIGATION'::text, 'TEMPORARY_TAKEDOWN_SELF_CHECK'::text]))
);
-- Create "report_evidence" table
CREATE TABLE "seller"."report_evidence" (
  "report_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "asset_id" text NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("report_id", "position"),
  CONSTRAINT "report_evidence_report_id_fkey" FOREIGN KEY ("report_id") REFERENCES "seller"."reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_evidence_asset_check" CHECK ((asset_id <> ''::text) AND (object_key <> ''::text)),
  CONSTRAINT "report_evidence_position_check" CHECK (("position" >= 0) AND ("position" < 5))
);
-- Drop index "suspensions_active" from table: "suspensions"
DROP INDEX "seller"."suspensions_active";
-- Modify "suspensions" table
-- 手動調整（只有這一段，atlas 產生的是 ADD COLUMN "source" text NOT NULL，套在已有 7a 處分列的 DB 上會失敗）：
-- 先以可為 NULL 的欄位加入，7a 留下的處分列都是管理員停權（回填 'ADMIN'），再設 NOT NULL 與約束。
ALTER TABLE "seller"."suspensions" ALTER COLUMN "reason" DROP NOT NULL, ADD COLUMN "source" text NULL, ADD COLUMN "violation_id" uuid NULL, ADD COLUMN "ends_at" timestamptz NULL, ADD CONSTRAINT "suspensions_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
UPDATE "seller"."suspensions" SET "source" = 'ADMIN';
ALTER TABLE "seller"."suspensions" ALTER COLUMN "source" SET NOT NULL, ADD CONSTRAINT "suspensions_source_check" CHECK (source = ANY (ARRAY['ADMIN'::text, 'VIOLATION'::text])), ADD CONSTRAINT "suspensions_source_fields_check" CHECK (((source = 'ADMIN'::text) AND (violation_id IS NULL) AND (reason IS NOT NULL) AND (ends_at IS NULL)) OR ((source = 'VIOLATION'::text) AND (violation_id IS NOT NULL) AND (reason IS NULL) AND (detail IS NULL) AND (ends_at IS NOT NULL) AND (ends_at > suspended_at)));
-- 手動調整：suspension_id 由 bigint IDENTITY 改成 UUIDv7（row-identity ADR：Java 不消費處分的 ID，它的下架快照以
-- period_id 為 scope）。Atlas 不產生 IDENTITY 到 uuid 的變更，所以重建這一欄；既有的列以 uuidv7() 回填。
DROP INDEX "seller"."suspensions_store";
ALTER TABLE "seller"."suspensions" ADD COLUMN "suspension_uuid" uuid NULL;
UPDATE "seller"."suspensions" SET "suspension_uuid" = uuidv7();
ALTER TABLE "seller"."suspensions" DROP CONSTRAINT "suspensions_pkey", DROP COLUMN "suspension_id";
ALTER TABLE "seller"."suspensions" RENAME COLUMN "suspension_uuid" TO "suspension_id";
ALTER TABLE "seller"."suspensions" ALTER COLUMN "suspension_id" SET NOT NULL, ADD PRIMARY KEY ("suspension_id");
CREATE INDEX "suspensions_store" ON "seller"."suspensions" ("store_id", "suspended_at" DESC, "suspension_id" DESC);
-- Create index "suspensions_admin_active" to table: "suspensions"
CREATE UNIQUE INDEX "suspensions_admin_active" ON "seller"."suspensions" ("store_id") WHERE ((source = 'ADMIN'::text) AND (lifted_at IS NULL));
-- Create index "suspensions_violation" to table: "suspensions"
CREATE UNIQUE INDEX "suspensions_violation" ON "seller"."suspensions" ("violation_id");
-- Create index "suspensions_violation_effective" to table: "suspensions"
CREATE INDEX "suspensions_violation_effective" ON "seller"."suspensions" ("ends_at") WHERE ((source = 'VIOLATION'::text) AND (lifted_at IS NULL));
