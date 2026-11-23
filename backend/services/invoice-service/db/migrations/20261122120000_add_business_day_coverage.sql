-- Create "invoice_business_day_coverage" table
CREATE TABLE "invoice_business_day_coverage" (
  "singleton" boolean NOT NULL DEFAULT true,
  "covered_from" date NOT NULL,
  "covered_to" date NOT NULL,
  "synced_at" timestamptz NOT NULL,
  CONSTRAINT "invoice_business_day_coverage_pkey" PRIMARY KEY ("singleton"),
  CONSTRAINT "chk_invbizcov_range" CHECK (covered_from <= covered_to),
  CONSTRAINT "chk_invbizcov_singleton" CHECK (singleton)
);
-- Set comment to table: "invoice_business_day_coverage"
COMMENT ON TABLE "invoice_business_day_coverage" IS '行事曆鏡像的涵蓋區間（單列）。由 ReplaceBusinessDayMirror 與假日清單同一交易覆蓋；沒有列代表鏡像從未被推送過。';
-- Set comment to table: "invoice_business_day_mirror"
COMMENT ON TABLE "invoice_business_day_mirror" IS '台灣國定假日唯讀鏡像。owner 是單體的 platform/bizcalendar；本表由單體呼叫 ReplaceBusinessDayMirror 整段覆蓋。禁止人工寫入、禁止 ADM 編輯、禁止在本服務加 seed migration。';
