-- 通報與案件分開（step10e-reports-brief R2、R3、R7，Will 2026-10-18；設計 step10e-3ca-design §2.2、§2.3）：新增案件（cases）、案件的轉報（case_referrals）、通報的補充
-- （report_supplements）與移動紀錄（report_case_moves）；通報改存結果（outcome）、案件與平台判定類別、分案，刪掉確認收件、嚴重度、調查與 FDA 轉報的欄位（嚴重度與轉報搬到案件）；
-- 註記可以掛在案件並記改嚴重度的改前改後；措施可以記來源案件；去重只比對還沒有結果的通報，匿名聯絡方式的清理改看結果的時間。
-- 這個檔案是 atlas migrate diff 產生的 DDL，在中間手寫了核對與資料轉換（開發資料，系統還沒上線）：先核對，任何一條成立就整個失敗並列出 ID、什麼都不改
-- （先例：20261018074115 的 OTHER 認定、20261015074826 的 ADMIN 關店）；再加新表與新欄位；轉換；最後刪舊欄位與舊約束、加新約束與索引。
-- 本機資料過不了核對時，重建本機的 hawkr database（DROP DATABASE 後 CREATE DATABASE，再 make migrate、make dev-seed；步驟見 seller 的 CARRYOVER「通報與案件」列）後重跑；
-- CI 的資料庫是新建的，不受影響。轉換不發事件。
--
-- 轉換規則（每筆舊通報）：
--   PENDING，沒確認收件                                   → 待分案（沒有案件、沒有結果）
--   PENDING 已確認收件，或 INVESTIGATING，類別不是 OTHER    → 進行中的案件，每個 (店家, 類別) 一個：嚴重度取成員中最嚴重的，開案時間與開案者取最早的確認收件（同時間取 id 小的），
--                                                          updated_at 取成員 updated_at 最晚的；通報的平台判定類別是自己的類別，分案時間與分案者是自己的確認收件；「調查中」不留
--   PENDING 已確認收件，或 INVESTIGATING，類別是 OTHER      → 待分案（確認收件與嚴重度不留：它們從沒判定過政策類別，這是丟掉值，不是寫錯的值）
--   VERIFIED                                              → 每筆一個已結案的案件（DECIDED、ACTION_TAKEN，嚴重度取自己的，開案取確認收件，結案取 resolved_*，說明取 resolution_note），通報的結果是 ACTION_TAKEN
--   DISMISSED                                             → 不開案件，結果 NO_VIOLATION（resolved_* 與說明保留；確認收件與嚴重度不留）
--   FDA 轉報                                               → case_referrals（所在案件；channel、reference_no、referred_at = fda_reported_at、admin_id = fda_reported_by_admin_id）
-- 既有措施的 source_case_id 都是 NULL（以前沒有案件，導出關係由認定推導）；所有通報 version = 1、discarded_submissions = 0。

-- 核對一：VERIFIED 而類別是 OTHER 的通報沒有誠實的案件類別（案件的類別不能是 OTHER，而「已採取措施」的通報一定在案件裡）。
-- 核對二：有 FDA 轉報、卻不會進任何案件的通報（DISMISSED，或 PENDING 而沒確認收件）：轉報掛在案件上。
-- 核對三：會合併進同一個進行中案件的通報裡有兩筆以上的 FDA 轉報：一個案件一筆轉報。
-- 核對四：配合要求（report_cooperation_requests）掛在不會進任何案件的通報上：3c-b 把配合要求轉成案件的詢問，這些列沒有去處。
DO $$
DECLARE
  ids text;
BEGIN
  SELECT string_agg(r.id::text, ', ' ORDER BY r.id) INTO ids
  FROM "seller"."reports" r WHERE r.status = 'VERIFIED' AND r.category = 'OTHER';
  IF ids IS NOT NULL THEN
    RAISE EXCEPTION '查證屬實而類別是 OTHER 的通報沒有對應的案件類別（案件的類別不能是 OTHER），請先人工處理這些通報或重建本機資料庫後重跑：%', ids;
  END IF;

  SELECT string_agg(r.id::text, ', ' ORDER BY r.id) INTO ids
  FROM "seller"."reports" r
  WHERE r.fda_reported_at IS NOT NULL AND (r.status = 'DISMISSED' OR (r.status = 'PENDING' AND r.acknowledged_at IS NULL));
  IF ids IS NOT NULL THEN
    RAISE EXCEPTION '有 FDA 轉報、卻不會進任何案件的通報（不成立，或待處理而沒確認收件）沒有地方放轉報紀錄（轉報掛在案件上），請先人工處理這些通報或重建本機資料庫後重跑：%', ids;
  END IF;

  SELECT string_agg(r.id::text, ', ' ORDER BY r.id) INTO ids
  FROM "seller"."reports" r
  WHERE r.fda_reported_at IS NOT NULL AND r.category <> 'OTHER'
    AND ((r.status = 'PENDING' AND r.acknowledged_at IS NOT NULL) OR r.status = 'INVESTIGATING')
    AND EXISTS (
      SELECT 1 FROM "seller"."reports" o
      WHERE o.store_id = r.store_id AND o.category = r.category AND o.id <> r.id AND o.fda_reported_at IS NOT NULL
        AND ((o.status = 'PENDING' AND o.acknowledged_at IS NOT NULL) OR o.status = 'INVESTIGATING'));
  IF ids IS NOT NULL THEN
    RAISE EXCEPTION '同一家店、同一個類別有兩筆以上的通報都記了 FDA 轉報，合併成一個進行中的案件後一個案件只能有一筆轉報，請先人工處理這些通報或重建本機資料庫後重跑：%', ids;
  END IF;

  SELECT string_agg(DISTINCT c.report_id::text, ', ' ORDER BY c.report_id::text) INTO ids
  FROM "seller"."report_cooperation_requests" c
  JOIN "seller"."reports" r ON r.id = c.report_id
  WHERE r.status = 'DISMISSED'
     OR (r.status = 'PENDING' AND r.acknowledged_at IS NULL)
     OR (r.status IN ('PENDING', 'INVESTIGATING') AND r.acknowledged_at IS NOT NULL AND r.category = 'OTHER');
  IF ids IS NOT NULL THEN
    RAISE EXCEPTION '要求賣家配合的紀錄掛在不會進任何案件的通報上（3c-b 把它轉成案件的詢問，這些列沒有去處），請先人工處理這些通報或重建本機資料庫後重跑：%', ids;
  END IF;
END
$$;
-- Create "cases" table
CREATE TABLE "seller"."cases" (
  "id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "category" text NOT NULL,
  "severity" text NOT NULL,
  "version" bigint NOT NULL DEFAULT 1,
  "opened_at" timestamptz NOT NULL,
  "opened_by_admin_id" bigint NOT NULL,
  "closed_at" timestamptz NULL,
  "closed_by_admin_id" bigint NULL,
  "close_reason" text NULL,
  "decision" text NULL,
  "decision_note" text NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "cases_category_key" UNIQUE ("id", "category"),
  CONSTRAINT "cases_report_key" UNIQUE ("id", "store_id", "category"),
  CONSTRAINT "cases_store_key" UNIQUE ("id", "store_id"),
  CONSTRAINT "cases_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "cases_admin_check" CHECK (opened_by_admin_id > 0),
  CONSTRAINT "cases_category_check" CHECK (category = ANY (ARRAY['FOOD_SAFETY'::text, 'PROHIBITED_GOODS'::text, 'UNLICENSED_SALE'::text, 'MISLEADING_INFO'::text, 'FAKE_CERTIFICATION'::text, 'COUNTERFEIT'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text])),
  CONSTRAINT "cases_close_check" CHECK (((closed_at IS NULL) AND (closed_by_admin_id IS NULL) AND (close_reason IS NULL) AND (decision IS NULL) AND (decision_note IS NULL)) OR ((closed_at >= opened_at) AND (closed_by_admin_id > 0) AND (close_reason = 'DECIDED'::text) AND (decision = ANY (ARRAY['ACTION_TAKEN'::text, 'NO_ACTION'::text])) AND ((decision_note IS NULL) OR ((char_length(decision_note) >= 1) AND (char_length(decision_note) <= 2000)))) OR ((closed_at >= opened_at) AND (closed_by_admin_id > 0) AND (close_reason = 'NO_REPORTS'::text) AND (decision IS NULL) AND (decision_note IS NULL))),
  CONSTRAINT "cases_severity_check" CHECK (severity = ANY (ARRAY['LOW'::text, 'MEDIUM'::text, 'HIGH'::text, 'CRITICAL'::text])),
  CONSTRAINT "cases_updated_check" CHECK (updated_at >= opened_at),
  CONSTRAINT "cases_version_check" CHECK (version >= 1)
);
-- Create index "cases_closed" to table: "cases"
CREATE INDEX "cases_closed" ON "seller"."cases" ("opened_at" DESC, "id" DESC) WHERE (closed_at IS NOT NULL);
-- Create index "cases_decided_food_safety" to table: "cases"
CREATE INDEX "cases_decided_food_safety" ON "seller"."cases" ("closed_at", "id") WHERE ((category = 'FOOD_SAFETY'::text) AND (decision = 'ACTION_TAKEN'::text));
-- Create index "cases_one_open" to table: "cases"
CREATE UNIQUE INDEX "cases_one_open" ON "seller"."cases" ("store_id", "category") WHERE (closed_at IS NULL);
-- Create index "cases_open" to table: "cases"
CREATE INDEX "cases_open" ON "seller"."cases" ("opened_at" DESC, "id" DESC) WHERE (closed_at IS NULL);
-- Create index "cases_opened" to table: "cases"
CREATE INDEX "cases_opened" ON "seller"."cases" ("opened_at" DESC, "id" DESC);
-- Create index "cases_store" to table: "cases"
CREATE INDEX "cases_store" ON "seller"."cases" ("store_id", "opened_at" DESC, "id" DESC);
-- Create "case_referrals" table
CREATE TABLE "seller"."case_referrals" (
  "case_id" uuid NOT NULL,
  "category" text NOT NULL,
  "channel" text NOT NULL,
  "reference_no" text NULL,
  "referred_at" timestamptz NOT NULL,
  "admin_id" bigint NOT NULL,
  PRIMARY KEY ("case_id"),
  CONSTRAINT "case_referrals_case_fkey" FOREIGN KEY ("case_id", "category") REFERENCES "seller"."cases" ("id", "category") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "case_referrals_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "case_referrals_category_check" CHECK (category = 'FOOD_SAFETY'::text),
  CONSTRAINT "case_referrals_channel_check" CHECK (channel = ANY (ARRAY['HOTLINE_1919'::text, 'ONLINE_FORM'::text, 'FAX_HEALTH_DEPT'::text, 'IN_PERSON'::text, 'OTHER'::text])),
  CONSTRAINT "case_referrals_reference_no_check" CHECK ((char_length(reference_no) >= 1) AND (char_length(reference_no) <= 100))
);
-- Modify "governance_notes" table
ALTER TABLE "seller"."governance_notes" ADD CONSTRAINT "governance_notes_owner_check" CHECK (num_nonnulls(case_id, measure_id) = 1), ADD CONSTRAINT "governance_notes_severity_check" CHECK (((severity_from IS NULL) = (severity_to IS NULL)) AND ((severity_from IS NULL) OR ((case_id IS NOT NULL) AND (severity_from <> severity_to) AND (severity_from = ANY (ARRAY['LOW'::text, 'MEDIUM'::text, 'HIGH'::text, 'CRITICAL'::text])) AND (severity_to = ANY (ARRAY['LOW'::text, 'MEDIUM'::text, 'HIGH'::text, 'CRITICAL'::text]))))), ALTER COLUMN "measure_id" DROP NOT NULL, ADD COLUMN "case_id" uuid NULL, ADD COLUMN "severity_from" text NULL, ADD COLUMN "severity_to" text NULL, ADD CONSTRAINT "governance_notes_case_id_fkey" FOREIGN KEY ("case_id") REFERENCES "seller"."cases" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "governance_notes_case" to table: "governance_notes"
CREATE INDEX "governance_notes_case" ON "seller"."governance_notes" ("case_id", "created_at", "note_id") WHERE (case_id IS NOT NULL);
-- Modify "measures" table
ALTER TABLE "seller"."measures" ADD COLUMN "source_case_id" uuid NULL, ADD CONSTRAINT "measures_source_case_fkey" FOREIGN KEY ("source_case_id", "store_id") REFERENCES "seller"."cases" ("id", "store_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "measures_source_case" to table: "measures"
CREATE INDEX "measures_source_case" ON "seller"."measures" ("source_case_id") WHERE (source_case_id IS NOT NULL);
-- Modify "reports" table：先加新欄位與 (id, store_id) 的唯一鍵（轉換與下面的複合外鍵要用）；舊欄位、舊約束與新約束在轉換之後。
ALTER TABLE "seller"."reports" ADD COLUMN "case_id" uuid NULL, ADD COLUMN "assessed_category" text NULL, ADD COLUMN "triaged_at" timestamptz NULL, ADD COLUMN "triaged_by_admin_id" bigint NULL, ADD COLUMN "outcome" text NULL, ADD COLUMN "discarded_submissions" integer NOT NULL DEFAULT 0, ADD COLUMN "version" bigint NOT NULL DEFAULT 1, ADD CONSTRAINT "reports_store_key" UNIQUE ("id", "store_id");
-- Create "report_case_moves" table
CREATE TABLE "seller"."report_case_moves" (
  "id" uuid NOT NULL,
  "report_id" uuid NOT NULL,
  "store_id" bigint NOT NULL,
  "from_case_id" uuid NOT NULL,
  "to_case_id" uuid NOT NULL,
  "admin_id" bigint NOT NULL,
  "moved_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "report_case_moves_from_fkey" FOREIGN KEY ("from_case_id", "store_id") REFERENCES "seller"."cases" ("id", "store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_case_moves_report_fkey" FOREIGN KEY ("report_id", "store_id") REFERENCES "seller"."reports" ("id", "store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_case_moves_to_fkey" FOREIGN KEY ("to_case_id", "store_id") REFERENCES "seller"."cases" ("id", "store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_case_moves_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "report_case_moves_distinct_check" CHECK (from_case_id <> to_case_id)
);
-- Create index "report_case_moves_from" to table: "report_case_moves"
CREATE INDEX "report_case_moves_from" ON "seller"."report_case_moves" ("from_case_id", "moved_at", "id");
-- Create index "report_case_moves_to" to table: "report_case_moves"
CREATE INDEX "report_case_moves_to" ON "seller"."report_case_moves" ("to_case_id", "moved_at", "id");
-- Create "report_supplements" table
CREATE TABLE "seller"."report_supplements" (
  "id" uuid NOT NULL,
  "report_id" uuid NOT NULL,
  "description" text NOT NULL,
  "matched_by" text NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "report_supplements_report_key" UNIQUE ("id", "report_id"),
  CONSTRAINT "report_supplements_report_id_fkey" FOREIGN KEY ("report_id") REFERENCES "seller"."reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_supplements_description_check" CHECK ((char_length(description) >= 1) AND (char_length(description) <= 2000)),
  CONSTRAINT "report_supplements_matched_by_check" CHECK (matched_by = ANY (ARRAY['PHONE'::text, 'EMAIL'::text, 'BOTH'::text]))
);
-- Create index "report_supplements_report" to table: "report_supplements"
CREATE INDEX "report_supplements_report" ON "seller"."report_supplements" ("report_id", "created_at", "id");
-- Modify "report_evidence" table
ALTER TABLE "seller"."report_evidence" DROP CONSTRAINT "report_evidence_position_check", ADD CONSTRAINT "report_evidence_position_check" CHECK ("position" >= 0), ADD COLUMN "supplement_id" uuid NULL, ADD CONSTRAINT "report_evidence_supplement_fkey" FOREIGN KEY ("supplement_id", "report_id") REFERENCES "seller"."report_supplements" ("id", "report_id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- 轉換一：進行中的案件。成員是 PENDING 已確認收件或 INVESTIGATING、類別不是 OTHER 的通報，每個 (店家, 類別) 一個案件。
WITH members AS (
  SELECT r.* FROM "seller"."reports" r
  WHERE r.category <> 'OTHER' AND ((r.status = 'PENDING' AND r.acknowledged_at IS NOT NULL) OR r.status = 'INVESTIGATING')
), grouped AS (
  SELECT m.store_id, m.category,
         (array_agg(m.severity ORDER BY CASE m.severity WHEN 'CRITICAL' THEN 3 WHEN 'HIGH' THEN 2 WHEN 'MEDIUM' THEN 1 ELSE 0 END DESC, m.id))[1] AS severity,
         (array_agg(m.acknowledged_at ORDER BY m.acknowledged_at, m.id))[1] AS opened_at,
         (array_agg(m.acknowledged_by_admin_id ORDER BY m.acknowledged_at, m.id))[1] AS opened_by_admin_id,
         max(m.updated_at) AS updated_at
  FROM members m
  GROUP BY m.store_id, m.category
), created AS (
  INSERT INTO "seller"."cases" ("id", "store_id", "category", "severity", "opened_at", "opened_by_admin_id", "updated_at")
  SELECT uuidv7(), g.store_id, g.category, g.severity, g.opened_at, g.opened_by_admin_id, GREATEST(g.updated_at, g.opened_at)
  FROM grouped g
  RETURNING "id", "store_id", "category"
)
UPDATE "seller"."reports" r
SET "case_id" = c.id, "assessed_category" = r.category, "triaged_at" = r.acknowledged_at, "triaged_by_admin_id" = r.acknowledged_by_admin_id
FROM created c, members m
WHERE m.id = r.id AND c.store_id = r.store_id AND c.category = r.category;
-- 轉換二：查證屬實的通報，每筆一個已結案的案件（決定採取措施）。
WITH verified AS MATERIALIZED (
  SELECT r.id AS report_id, uuidv7() AS case_id, r.store_id, r.category, r.severity, r.acknowledged_at, r.acknowledged_by_admin_id,
         r.resolved_at, r.resolved_by_admin_id, r.resolution_note
  FROM "seller"."reports" r WHERE r.status = 'VERIFIED'
), created AS (
  INSERT INTO "seller"."cases" ("id", "store_id", "category", "severity", "opened_at", "opened_by_admin_id", "closed_at", "closed_by_admin_id", "close_reason", "decision", "decision_note", "updated_at")
  SELECT v.case_id, v.store_id, v.category, v.severity, v.acknowledged_at, v.acknowledged_by_admin_id, v.resolved_at, v.resolved_by_admin_id, 'DECIDED', 'ACTION_TAKEN', v.resolution_note, v.resolved_at
  FROM verified v
  RETURNING "id"
)
UPDATE "seller"."reports" r
SET "case_id" = v.case_id, "assessed_category" = r.category, "triaged_at" = r.acknowledged_at, "triaged_by_admin_id" = r.acknowledged_by_admin_id, "outcome" = 'ACTION_TAKEN'
FROM verified v
WHERE v.report_id = r.id AND EXISTS (SELECT 1 FROM created c WHERE c.id = v.case_id);
-- 轉換三：不成立的通報不開案件，結果是未違規。
UPDATE "seller"."reports" SET "outcome" = 'NO_VIOLATION' WHERE "status" = 'DISMISSED';
-- 轉換四：FDA 轉報搬到案件（核對二、三保證每筆都有案件、每個案件最多一筆）。
INSERT INTO "seller"."case_referrals" ("case_id", "category", "channel", "reference_no", "referred_at", "admin_id")
SELECT r.case_id, r.category, r.fda_channel, r.fda_reference_no, r.fda_reported_at, r.fda_reported_by_admin_id
FROM "seller"."reports" r WHERE r.fda_reported_at IS NOT NULL;
-- Drop index "reports_buyer_daily" from table: "reports"
DROP INDEX "seller"."reports_buyer_daily";
-- Drop index "reports_contact_retention" from table: "reports"
DROP INDEX "seller"."reports_contact_retention";
-- Drop index "reports_email_weekly" from table: "reports"
DROP INDEX "seller"."reports_email_weekly";
-- Drop index "reports_phone_weekly" from table: "reports"
DROP INDEX "seller"."reports_phone_weekly";
-- Modify "reports" table：刪舊欄位與舊約束（確認收件、嚴重度、調查與 FDA 轉報），加新約束與複合外鍵（轉換之後才加：資料要先符合）。
ALTER TABLE "seller"."reports" DROP CONSTRAINT "reports_acknowledged_check", DROP CONSTRAINT "reports_fda_channel_check", DROP CONSTRAINT "reports_fda_check", DROP CONSTRAINT "reports_severity_check", DROP CONSTRAINT "reports_status_check", DROP CONSTRAINT "reports_status_times_check", DROP CONSTRAINT "reports_admin_check", ADD CONSTRAINT "reports_admin_check" CHECK ((triaged_by_admin_id > 0) AND (resolved_by_admin_id > 0)), DROP CONSTRAINT "reports_resolved_check", ADD CONSTRAINT "reports_resolved_check" CHECK (((outcome IS NULL) = (resolved_at IS NULL)) AND ((resolved_at IS NULL) = (resolved_by_admin_id IS NULL)) AND ((resolution_note IS NULL) OR ((resolved_at IS NOT NULL) AND ((char_length(resolution_note) >= 1) AND (char_length(resolution_note) <= 2000)))) AND ((resolved_at IS NULL) OR (resolved_at >= created_at))), ADD CONSTRAINT "reports_action_taken_check" CHECK ((outcome IS DISTINCT FROM 'ACTION_TAKEN'::text) OR (case_id IS NOT NULL)), ADD CONSTRAINT "reports_counts_check" CHECK ((discarded_submissions >= 0) AND (version >= 1)), ADD CONSTRAINT "reports_outcome_check" CHECK (outcome = ANY (ARRAY['ACTION_TAKEN'::text, 'NO_VIOLATION'::text, 'INSUFFICIENT'::text, 'DUPLICATE'::text, 'ABUSIVE'::text])), ADD CONSTRAINT "reports_triage_check" CHECK (((case_id IS NULL) = (assessed_category IS NULL)) AND ((case_id IS NULL) = (triaged_at IS NULL)) AND ((triaged_at IS NULL) = (triaged_by_admin_id IS NULL)) AND ((triaged_at IS NULL) OR (triaged_at >= created_at))), DROP COLUMN "status", DROP COLUMN "severity", DROP COLUMN "acknowledged_at", DROP COLUMN "acknowledged_by_admin_id", DROP COLUMN "investigating_at", DROP COLUMN "fda_reported_at", DROP COLUMN "fda_channel", DROP COLUMN "fda_reference_no", DROP COLUMN "fda_reported_by_admin_id", ADD CONSTRAINT "reports_case_fkey" FOREIGN KEY ("case_id", "store_id", "assessed_category") REFERENCES "seller"."cases" ("id", "store_id", "category") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "reports_buyer_daily" to table: "reports"
CREATE UNIQUE INDEX "reports_buyer_daily" ON "seller"."reports" ("reporter_buyer_id", "store_id", "category", (((created_at AT TIME ZONE 'Asia/Taipei'::text))::date)) WHERE ((reporter_buyer_id IS NOT NULL) AND (outcome IS NULL));
-- Create index "reports_contact_retention" to table: "reports"
CREATE INDEX "reports_contact_retention" ON "seller"."reports" ("resolved_at") WHERE ((reporter_buyer_id IS NULL) AND (contact_erased_at IS NULL) AND (resolved_at IS NOT NULL));
-- Create index "reports_email_weekly" to table: "reports"
CREATE UNIQUE INDEX "reports_email_weekly" ON "seller"."reports" ("reporter_email", "store_id", (date_trunc('week'::text, (created_at AT TIME ZONE 'Asia/Taipei'::text)))) WHERE ((reporter_email IS NOT NULL) AND (outcome IS NULL));
-- Create index "reports_phone_weekly" to table: "reports"
CREATE UNIQUE INDEX "reports_phone_weekly" ON "seller"."reports" ("reporter_phone", "store_id", (date_trunc('week'::text, (created_at AT TIME ZONE 'Asia/Taipei'::text)))) WHERE ((reporter_phone IS NOT NULL) AND (outcome IS NULL));
-- Create index "reports_awaiting_triage" to table: "reports"
CREATE INDEX "reports_awaiting_triage" ON "seller"."reports" ("created_at", "id") WHERE ((case_id IS NULL) AND (outcome IS NULL));
-- Create index "reports_case" to table: "reports"
CREATE INDEX "reports_case" ON "seller"."reports" ("case_id", "created_at", "id") WHERE (case_id IS NOT NULL);
