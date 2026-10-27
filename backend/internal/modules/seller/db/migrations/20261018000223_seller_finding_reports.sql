-- 認定與通報改成多對多（finding_reports）：先建表、把既有的單筆來源通報搬進去，再刪 violations.source_report_id（連同它的索引 violations_source_report）。
-- Create "finding_reports" table
CREATE TABLE "seller"."finding_reports" (
  "violation_id" uuid NOT NULL,
  "report_id" uuid NOT NULL,
  PRIMARY KEY ("violation_id", "report_id"),
  CONSTRAINT "finding_reports_report_id_fkey" FOREIGN KEY ("report_id") REFERENCES "seller"."reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "finding_reports_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "finding_reports_report" to table: "finding_reports"
CREATE INDEX "finding_reports_report" ON "seller"."finding_reports" ("report_id");
-- Copy existing single source reports
INSERT INTO "seller"."finding_reports" ("violation_id", "report_id")
SELECT "id", "source_report_id" FROM "seller"."violations" WHERE "source_report_id" IS NOT NULL;
-- Modify "violations" table
ALTER TABLE "seller"."violations" DROP COLUMN "source_report_id";
