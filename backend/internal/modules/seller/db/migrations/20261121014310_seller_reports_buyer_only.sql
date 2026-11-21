-- 匿名檢舉入口與聯絡方式已移除（ADR 2026-10-18 的 2026-11-21 修訂）：系統從未上線，先刪既有的匿名通報與依附它的列，再收緊約束。
DELETE FROM "seller"."evidence_accesses" WHERE report_id IN (SELECT id FROM "seller"."reports" WHERE reporter_buyer_id IS NULL);
DELETE FROM "seller"."report_evidence" WHERE report_id IN (SELECT id FROM "seller"."reports" WHERE reporter_buyer_id IS NULL);
DELETE FROM "seller"."report_supplements" WHERE report_id IN (SELECT id FROM "seller"."reports" WHERE reporter_buyer_id IS NULL);
DELETE FROM "seller"."report_case_moves" WHERE report_id IN (SELECT id FROM "seller"."reports" WHERE reporter_buyer_id IS NULL);
DELETE FROM "seller"."finding_reports" WHERE report_id IN (SELECT id FROM "seller"."reports" WHERE reporter_buyer_id IS NULL);
DELETE FROM "seller"."reports" WHERE reporter_buyer_id IS NULL;
-- Modify "report_supplements" table
ALTER TABLE "seller"."report_supplements" DROP CONSTRAINT "report_supplements_matched_by_check", DROP COLUMN "matched_by";
-- Drop index "reports_buyer_daily" from table: "reports"
DROP INDEX "seller"."reports_buyer_daily";
-- Drop index "reports_contact_cap" from table: "reports"
DROP INDEX "seller"."reports_contact_cap";
-- Drop index "reports_reporter" from table: "reports"
DROP INDEX "seller"."reports_reporter";
-- Modify "reports" table
ALTER TABLE "seller"."reports" DROP CONSTRAINT "reports_reporter_email_check", DROP CONSTRAINT "reports_reporter_name_check", DROP CONSTRAINT "reports_reporter_phone_check", DROP CONSTRAINT "reports_counts_check", ADD CONSTRAINT "reports_counts_check" CHECK (version >= 1), DROP CONSTRAINT "reports_reporter_check", ADD CONSTRAINT "reports_reporter_check" CHECK (reporter_buyer_id > 0), ALTER COLUMN "reporter_buyer_id" SET NOT NULL, DROP COLUMN "reporter_phone", DROP COLUMN "reporter_email", DROP COLUMN "reporter_name", DROP COLUMN "contact_erased_at", DROP COLUMN "discarded_submissions";
-- Create index "reports_buyer_daily" to table: "reports"
CREATE UNIQUE INDEX "reports_buyer_daily" ON "seller"."reports" ("reporter_buyer_id", "store_id", "category", (((created_at AT TIME ZONE 'Asia/Taipei'::text))::date)) WHERE (outcome IS NULL);
-- Create index "reports_reporter" to table: "reports"
CREATE INDEX "reports_reporter" ON "seller"."reports" ("reporter_buyer_id", "created_at" DESC, "id" DESC);
