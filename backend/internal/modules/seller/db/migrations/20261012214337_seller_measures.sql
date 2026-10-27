-- 措施取代處分表：違規 1:1 回填成措施（id 沿用，認定留在 violations），申訴欄位搬到 measure_requests，版本表與註記改名搬到
-- measure_*，身分快照改掛措施。停權的效果不再存成另一張表（suspensions 刪除）：措施的狀態、開始時間與 ends_at 是唯一來源；給賣家的理由與結束時間的目前值存在 measures，measure_revisions 是歷史快照（SQL:2011 系統版本表）。
-- 轉換前先核對，對不上就整個 migration 失敗並列出 ID，不寫出錯誤的歷史（先例：20261011174237）。
DO $$
DECLARE
  admin_rows text;
  mismatched text;
BEGIN
  -- 管理員停權列（含已解除的）沒有誠實可用的 risk_ground：舊的三個原因都對不到條款第 17.1、17.5 條的風險類型，
  -- 為它新增依據值會永久改變契約。既有資料只有本機與測試資料，請先人工清掉這些列（或重建本機資料庫）再重跑。
  SELECT string_agg(s.suspension_id::text, ', ' ORDER BY s.suspension_id) INTO admin_rows
  FROM "seller"."suspensions" s WHERE s.source = 'ADMIN';
  IF admin_rows IS NOT NULL THEN
    RAISE EXCEPTION 'suspensions 有管理員停權列，無法轉成措施（沒有對應的風險依據），請先人工處理這些列後重跑：%', admin_rows;
  END IF;
  -- 違規的處分列必須與違規一致：停權的違規恰好一筆處分，結束時間等於目前版本的 sanction_ends_at 而且不為空（依據認定的停權一定有結束時間），解除與否等於違規是否撤銷；
  -- 警告沒有處分列。
  SELECT string_agg(v.id::text, ', ' ORDER BY v.id) INTO mismatched
  FROM "seller"."violations" v
  LEFT JOIN "seller"."suspensions" s ON s.violation_id = v.id
  LEFT JOIN "seller"."violation_revisions" r ON r.violation_id = v.id AND r.revision = v.current_revision
  WHERE (v.penalty = 'SUSPENSION' AND (s.suspension_id IS NULL OR r.sanction_ends_at IS NULL OR s.ends_at IS DISTINCT FROM r.sanction_ends_at
         OR (s.lifted_at IS NOT NULL) <> (v.status = 'REVOKED')))
     OR (v.penalty = 'WARNING' AND (s.suspension_id IS NOT NULL OR r.sanction_ends_at IS NOT NULL));
  IF mismatched IS NOT NULL THEN
    RAISE EXCEPTION '違規與它的處分列不一致（結束時間、解除狀態或處分列的有無），回填會寫出錯誤的歷史，請先人工補齊：%', mismatched;
  END IF;
END
$$;
-- Create tables of the measure model
-- Create "measure_findings" table
CREATE TABLE "seller"."measure_findings" (
  "measure_id" uuid NOT NULL,
  "violation_id" uuid NOT NULL,
  PRIMARY KEY ("measure_id", "violation_id")
);
-- Create index "measure_findings_violation" to table: "measure_findings"
CREATE INDEX "measure_findings_violation" ON "seller"."measure_findings" ("violation_id");
-- Create "measure_identities" table
CREATE TABLE "seller"."measure_identities" (
  "measure_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "key_id" text NOT NULL,
  "hash" bytea NOT NULL,
  PRIMARY KEY ("measure_id", "kind", "hash"),
  CONSTRAINT "measure_identities_hash_check" CHECK (octet_length(hash) = 32),
  CONSTRAINT "measure_identities_key_check" CHECK (key_id <> ''::text),
  CONSTRAINT "measure_identities_kind_check" CHECK (kind = ANY (ARRAY['ID_CARD'::text, 'TAX_ID'::text, 'BANK_ACCOUNT'::text]))
);
-- Create index "measure_identities_hash" to table: "measure_identities"
CREATE INDEX "measure_identities_hash" ON "seller"."measure_identities" ("hash");
-- Create "measure_notes" table
CREATE TABLE "seller"."measure_notes" (
  "note_id" uuid NOT NULL,
  "measure_id" uuid NOT NULL,
  "admin_id" bigint NOT NULL,
  "body" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("note_id"),
  CONSTRAINT "measure_notes_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "measure_notes_body_check" CHECK ((char_length(body) >= 1) AND (char_length(body) <= 2000))
);
-- Create index "measure_notes_measure" to table: "measure_notes"
CREATE INDEX "measure_notes_measure" ON "seller"."measure_notes" ("measure_id", "created_at", "note_id");
-- Create "measure_request_evidence" table
CREATE TABLE "seller"."measure_request_evidence" (
  "request_id" uuid NOT NULL,
  "position" integer NOT NULL,
  "asset_id" text NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("request_id", "position"),
  CONSTRAINT "measure_request_evidence_asset_check" CHECK ((asset_id <> ''::text) AND (object_key <> ''::text)),
  CONSTRAINT "measure_request_evidence_position_check" CHECK (("position" >= 0) AND ("position" < 5))
);
-- Create "measure_requests" table
CREATE TABLE "seller"."measure_requests" (
  "id" uuid NOT NULL,
  "measure_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "state" text NOT NULL,
  "reason" text NOT NULL,
  "revision" integer NOT NULL,
  "submitted_at" timestamptz NOT NULL,
  "decision_reason" text NULL,
  "decided_by_admin_id" bigint NULL,
  "decided_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "measure_requests_decision_check" CHECK (((state = 'PENDING'::text) AND (decision_reason IS NULL) AND (decided_by_admin_id IS NULL) AND (decided_at IS NULL)) OR ((state = ANY (ARRAY['ACCEPTED'::text, 'REJECTED'::text])) AND (decision_reason IS NOT NULL) AND ((char_length(decision_reason) >= 1) AND (char_length(decision_reason) <= 2000)) AND (decided_by_admin_id IS NOT NULL) AND (decided_by_admin_id > 0) AND (decided_at IS NOT NULL) AND (decided_at >= submitted_at)) OR ((state = 'CLOSED'::text) AND (decision_reason IS NULL) AND (decided_by_admin_id IS NOT NULL) AND (decided_by_admin_id > 0) AND (decided_at IS NOT NULL) AND (decided_at >= submitted_at))),
  CONSTRAINT "measure_requests_kind_check" CHECK (kind = 'APPEAL'::text),
  CONSTRAINT "measure_requests_reason_check" CHECK ((char_length(reason) >= 1) AND (char_length(reason) <= 2000)),
  CONSTRAINT "measure_requests_state_check" CHECK (state = ANY (ARRAY['PENDING'::text, 'ACCEPTED'::text, 'REJECTED'::text, 'CLOSED'::text]))
);
-- Create index "measure_requests_one_appeal" to table: "measure_requests"
CREATE UNIQUE INDEX "measure_requests_one_appeal" ON "seller"."measure_requests" ("measure_id") WHERE (kind = 'APPEAL'::text);
-- Create index "measure_requests_one_pending" to table: "measure_requests"
CREATE UNIQUE INDEX "measure_requests_one_pending" ON "seller"."measure_requests" ("measure_id") WHERE (state = 'PENDING'::text);
-- Create index "measure_requests_pending" to table: "measure_requests"
CREATE INDEX "measure_requests_pending" ON "seller"."measure_requests" ("submitted_at", "id") WHERE (state = 'PENDING'::text);
-- Create "measure_revisions" table
CREATE TABLE "seller"."measure_revisions" (
  "measure_id" uuid NOT NULL,
  "revision" integer NOT NULL,
  "statement_to_seller" text NOT NULL,
  "ends_at" timestamptz NULL,
  "admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("measure_id", "revision"),
  CONSTRAINT "measure_revisions_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "measure_revisions_revision_check" CHECK (revision >= 1),
  CONSTRAINT "measure_revisions_statement_check" CHECK ((char_length(statement_to_seller) >= 1) AND (char_length(statement_to_seller) <= 2000))
);
-- Create "measures" table
CREATE TABLE "seller"."measures" (
  "id" uuid NOT NULL,
  "account_id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "kind" text NOT NULL,
  "basis" text NOT NULL,
  "risk_ground" text NULL,
  "lead_finding_id" uuid NULL,
  "review_by" timestamptz NULL,
  "statement_to_seller" text NOT NULL,
  "ends_at" timestamptz NULL,
  "status" text NOT NULL DEFAULT 'ENFORCED',
  "version" bigint NOT NULL DEFAULT 1,
  "current_revision" integer NOT NULL,
  "request_id" uuid NULL,
  "request_hash" bytea NULL,
  "created_by_admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  "revoked_reason" text NULL,
  "revoked_by_admin_id" bigint NULL,
  "revoked_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "measures_admin_check" CHECK ((created_by_admin_id > 0) AND (revoked_by_admin_id > 0)),
  CONSTRAINT "measures_basis_check" CHECK (((basis = 'RISK'::text) = (risk_ground IS NOT NULL)) AND ((basis = 'VIOLATION'::text) = (lead_finding_id IS NOT NULL)) AND ((basis = 'RISK'::text) = (review_by IS NOT NULL)) AND ((basis <> 'RISK'::text) OR (kind = 'SUSPENSION'::text))),
  CONSTRAINT "measures_basis_value_check" CHECK (basis = ANY (ARRAY['VIOLATION'::text, 'RISK'::text])),
  CONSTRAINT "measures_kind_check" CHECK (kind = ANY (ARRAY['WARNING'::text, 'SUSPENSION'::text])),
  CONSTRAINT "measures_period_check" CHECK (((kind <> 'WARNING'::text) OR (ends_at IS NULL)) AND ((basis <> 'VIOLATION'::text) OR (kind <> 'SUSPENSION'::text) OR (ends_at IS NOT NULL))),
  CONSTRAINT "measures_request_check" CHECK (((request_id IS NULL) = (request_hash IS NULL)) AND ((request_hash IS NULL) OR (octet_length(request_hash) = 32))),
  CONSTRAINT "measures_review_check" CHECK ((review_by IS NULL) OR (review_by > created_at)),
  CONSTRAINT "measures_revision_check" CHECK (current_revision >= 1),
  CONSTRAINT "measures_revoked_check" CHECK (((status = 'REVOKED'::text) = (revoked_at IS NOT NULL)) AND ((revoked_at IS NULL) = (revoked_reason IS NULL)) AND ((revoked_at IS NULL) = (revoked_by_admin_id IS NULL)) AND ((revoked_at IS NULL) OR (revoked_at >= created_at))),
  CONSTRAINT "measures_revoked_reason_check" CHECK (revoked_reason = ANY (ARRAY['APPEAL_ACCEPTED'::text, 'ADMIN_CORRECTION'::text, 'EXTERNAL_DECISION'::text, 'OTHER'::text, 'FINDING_REVOKED'::text])),
  CONSTRAINT "measures_risk_ground_check" CHECK (risk_ground = ANY (ARRAY['TRANSACTION'::text, 'FOOD_SAFETY'::text, 'FRAUD'::text, 'PERSONAL_DATA'::text, 'INFRINGEMENT'::text, 'SECURITY'::text, 'FULFILLMENT'::text, 'EXTERNAL_ORDER'::text])),
  CONSTRAINT "measures_statement_check" CHECK ((char_length(statement_to_seller) >= 1) AND (char_length(statement_to_seller) <= 2000)),
  CONSTRAINT "measures_status_check" CHECK (status = ANY (ARRAY['ENFORCED'::text, 'REVOKED'::text])),
  CONSTRAINT "measures_version_check" CHECK (version >= 1)
);
-- Create index "measures_account" to table: "measures"
CREATE INDEX "measures_account" ON "seller"."measures" ("account_id", "created_at" DESC, "id" DESC);
-- Create index "measures_active_open" to table: "measures"
CREATE INDEX "measures_active_open" ON "seller"."measures" ("created_at") WHERE ((status = 'ENFORCED'::text) AND (kind = 'SUSPENSION'::text) AND (ends_at IS NULL));
-- Create index "measures_active_timed" to table: "measures"
CREATE INDEX "measures_active_timed" ON "seller"."measures" ("ends_at", "created_at") WHERE ((status = 'ENFORCED'::text) AND (kind = 'SUSPENSION'::text) AND (ends_at IS NOT NULL));
-- Create index "measures_created" to table: "measures"
CREATE INDEX "measures_created" ON "seller"."measures" ("created_at" DESC, "id" DESC);
-- Create index "measures_enforced_suspension" to table: "measures"
CREATE INDEX "measures_enforced_suspension" ON "seller"."measures" ("account_id") WHERE ((status = 'ENFORCED'::text) AND (kind = 'SUSPENSION'::text));
-- Create index "measures_request" to table: "measures"
CREATE UNIQUE INDEX "measures_request" ON "seller"."measures" ("created_by_admin_id", "request_id") WHERE (request_id IS NOT NULL);
-- Create index "measures_review" to table: "measures"
CREATE INDEX "measures_review" ON "seller"."measures" ("review_by") WHERE ((status = 'ENFORCED'::text) AND (basis = 'RISK'::text));
-- Backfill "measures"：違規 1:1，id、版本、時間與撤銷紀錄照搬；違規自己是主要依據；給賣家的理由與結束時間取自目前那一版
-- （目前值在 measures，measure_revisions 是含目前那一版的歷史快照；LEFT JOIN 讓缺少目前版本的違規以 NOT NULL 失敗，不靜默漏掉）
INSERT INTO "seller"."measures" ("id", "account_id", "store_id", "kind", "basis", "lead_finding_id", "statement_to_seller", "ends_at", "status", "version",
  "current_revision", "created_by_admin_id", "created_at", "updated_at", "revoked_reason", "revoked_by_admin_id", "revoked_at")
SELECT v."id", st."account_id", v."store_id", v."penalty", 'VIOLATION', v."id", r."statement_to_seller", r."sanction_ends_at", v."status", v."version",
  v."current_revision", v."created_by_admin_id", v."created_at", v."updated_at", v."revoked_reason", v."revoked_by_admin_id", v."revoked_at"
FROM "seller"."violations" v
JOIN "seller"."stores" st ON st."store_id" = v."store_id"
LEFT JOIN "seller"."violation_revisions" r ON r."violation_id" = v."id" AND r."revision" = v."current_revision";
-- Backfill "measure_findings"：每個違規是自己措施的依據
INSERT INTO "seller"."measure_findings" ("measure_id", "violation_id") SELECT "id", "id" FROM "seller"."violations";
-- Backfill "measure_revisions"
INSERT INTO "seller"."measure_revisions" ("measure_id", "revision", "statement_to_seller", "ends_at", "admin_id", "created_at")
SELECT "violation_id", "revision", "statement_to_seller", "sanction_ends_at", "admin_id", "created_at" FROM "seller"."violation_revisions";
-- Backfill "measure_notes"
INSERT INTO "seller"."measure_notes" ("note_id", "measure_id", "admin_id", "body", "created_at")
SELECT "note_id", "violation_id", "admin_id", "body", "created_at" FROM "seller"."violation_notes";
-- Backfill "measure_requests"：申訴欄位搬成請求（id 沿用違規 ID）；接受申訴舊規則允許不附理由，條款第 18.4 條要求裁定必附理由，
-- 補上照實描述的佔位文字
INSERT INTO "seller"."measure_requests" ("id", "measure_id", "kind", "state", "reason", "revision", "submitted_at",
  "decision_reason", "decided_by_admin_id", "decided_at")
SELECT v."id", v."id", 'APPEAL', v."appeal_status", v."appeal_reason", v."appeal_revision", v."appeal_submitted_at",
  CASE WHEN v."appeal_status" = 'PENDING' THEN NULL
       ELSE COALESCE(v."appeal_decision_reason", '（裁定時未記錄理由）') END,
  v."appeal_decided_by_admin_id", v."appeal_decided_at"
FROM "seller"."violations" v
WHERE v."appeal_status" IS NOT NULL;
-- Backfill "measure_request_evidence"
INSERT INTO "seller"."measure_request_evidence" ("request_id", "position", "asset_id", "object_key")
SELECT "violation_id", "position", "asset_id", "object_key" FROM "seller"."appeal_evidence";
-- Backfill "measure_identities"：身分快照改掛措施（違規的處分列 ID 換成措施 ID）
INSERT INTO "seller"."measure_identities" ("measure_id", "kind", "key_id", "hash")
SELECT s."violation_id", si."kind", si."key_id", si."hash"
FROM "seller"."suspension_identities" si
JOIN "seller"."suspensions" s ON s."suspension_id" = si."suspension_id";
-- Backfill "evidence_accesses"：申訴佐證的讀取紀錄改指請求（請求 ID 等於違規 ID）
ALTER TABLE "seller"."evidence_accesses" ADD COLUMN "request_id" uuid NULL;
UPDATE "seller"."evidence_accesses" SET "request_id" = "violation_id" WHERE "violation_id" IS NOT NULL;
-- Backfill "violations"：帳號取自店家；每個違規的建立者是自己的措施
ALTER TABLE "seller"."violations" ADD COLUMN "account_id" bigint NULL, ADD COLUMN "origin_measure_id" uuid NULL;
UPDATE "seller"."violations" v SET "account_id" = st."account_id", "origin_measure_id" = v."id"
FROM "seller"."stores" st WHERE st."store_id" = v."store_id";
-- Modify "evidence_accesses" table
ALTER TABLE "seller"."evidence_accesses" DROP CONSTRAINT "evidence_accesses_owner_check", ADD CONSTRAINT "evidence_accesses_owner_check" CHECK (num_nonnulls(report_id, request_id) = 1), DROP COLUMN "violation_id";
-- Create index "evidence_accesses_request" to table: "evidence_accesses"
CREATE INDEX "evidence_accesses_request" ON "seller"."evidence_accesses" ("request_id", "accessed_at" DESC) WHERE (request_id IS NOT NULL);
-- Modify "suspension_identities" table
ALTER TABLE "seller"."suspension_identities" DROP CONSTRAINT "suspension_identities_suspension_id_fkey";
-- Modify "suspensions" table
ALTER TABLE "seller"."suspensions" DROP CONSTRAINT "suspensions_store_id_fkey", DROP CONSTRAINT "suspensions_violation_id_fkey";
-- Modify "appeal_evidence" table
ALTER TABLE "seller"."appeal_evidence" DROP CONSTRAINT "appeal_evidence_violation_id_fkey";
-- Modify "violation_notes" table
ALTER TABLE "seller"."violation_notes" DROP CONSTRAINT "violation_notes_violation_id_fkey";
-- Modify "violation_revisions" table
ALTER TABLE "seller"."violation_revisions" DROP CONSTRAINT "violation_revisions_violation_id_fkey";
-- Drop index "violations_escalation" from table: "violations"
DROP INDEX "seller"."violations_escalation";
-- Modify "violations" table
ALTER TABLE "seller"."violations" DROP CONSTRAINT "violations_appeal_check", DROP CONSTRAINT "violations_appeal_revoked_check", DROP CONSTRAINT "violations_appeal_status_check", DROP CONSTRAINT "violations_penalty_check", DROP CONSTRAINT "violations_revision_check", DROP CONSTRAINT "violations_version_check", DROP COLUMN "penalty", DROP COLUMN "version", DROP COLUMN "appeal_status", DROP COLUMN "appeal_reason", DROP COLUMN "appeal_submitted_at", DROP COLUMN "appeal_decision_reason", DROP COLUMN "appeal_decided_by_admin_id", DROP COLUMN "appeal_decided_at", DROP COLUMN "current_revision", DROP COLUMN "appeal_revision", ALTER COLUMN "account_id" SET NOT NULL, ALTER COLUMN "origin_measure_id" SET NOT NULL;
-- Create index "violations_escalation" to table: "violations"
CREATE INDEX "violations_escalation" ON "seller"."violations" ("account_id", "created_at") WHERE ((status = 'ENFORCED'::text) AND (COALESCE(escalated_from, severity) = 'MEDIUM'::text));
-- Create index "violations_account" to table: "violations"
CREATE INDEX "violations_account" ON "seller"."violations" ("account_id", "created_at" DESC, "id" DESC);
-- Modify "evidence_accesses" table
ALTER TABLE "seller"."evidence_accesses" ADD CONSTRAINT "evidence_accesses_request_fkey" FOREIGN KEY ("request_id", "position") REFERENCES "seller"."measure_request_evidence" ("request_id", "position") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "stores" table
ALTER TABLE "seller"."stores" ADD CONSTRAINT "stores_store_account_key" UNIQUE ("store_id", "account_id");
-- Modify "violations" table
ALTER TABLE "seller"."violations" DROP CONSTRAINT "violations_store_id_fkey", ADD CONSTRAINT "violations_store_account_fkey" FOREIGN KEY ("store_id", "account_id") REFERENCES "seller"."stores" ("store_id", "account_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "violations" table
ALTER TABLE "seller"."violations" ADD CONSTRAINT "violations_origin_measure_fkey" FOREIGN KEY ("origin_measure_id", "id") REFERENCES "seller"."measure_findings" ("measure_id", "violation_id") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED;
-- Modify "measure_findings" table
ALTER TABLE "seller"."measure_findings" ADD CONSTRAINT "measure_findings_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "measure_findings_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "measure_identities" table
ALTER TABLE "seller"."measure_identities" ADD CONSTRAINT "measure_identities_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "measure_notes" table
ALTER TABLE "seller"."measure_notes" ADD CONSTRAINT "measure_notes_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "measure_request_evidence" table
ALTER TABLE "seller"."measure_request_evidence" ADD CONSTRAINT "measure_request_evidence_request_id_fkey" FOREIGN KEY ("request_id") REFERENCES "seller"."measure_requests" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "measure_requests" table
ALTER TABLE "seller"."measure_requests" ADD CONSTRAINT "measure_requests_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION, ADD CONSTRAINT "measure_requests_revision_fkey" FOREIGN KEY ("measure_id", "revision") REFERENCES "seller"."measure_revisions" ("measure_id", "revision") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "measure_revisions" table
ALTER TABLE "seller"."measure_revisions" ADD CONSTRAINT "measure_revisions_measure_id_fkey" FOREIGN KEY ("measure_id") REFERENCES "seller"."measures" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "measures" table
ALTER TABLE "seller"."measures" ADD CONSTRAINT "measures_current_revision_fkey" FOREIGN KEY ("id", "current_revision") REFERENCES "seller"."measure_revisions" ("measure_id", "revision") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED, ADD CONSTRAINT "measures_lead_finding_fkey" FOREIGN KEY ("id", "lead_finding_id") REFERENCES "seller"."measure_findings" ("measure_id", "violation_id") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED, ADD CONSTRAINT "measures_store_account_fkey" FOREIGN KEY ("store_id", "account_id") REFERENCES "seller"."stores" ("store_id", "account_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Drop "appeal_evidence" table
DROP TABLE "seller"."appeal_evidence";
-- Drop "suspension_identities" table
DROP TABLE "seller"."suspension_identities";
-- Drop "suspensions" table
DROP TABLE "seller"."suspensions";
-- Drop "violation_notes" table
DROP TABLE "seller"."violation_notes";
-- Drop "violation_revisions" table
DROP TABLE "seller"."violation_revisions";
