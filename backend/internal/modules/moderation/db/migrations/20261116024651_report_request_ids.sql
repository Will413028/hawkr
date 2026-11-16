-- Modify "reports" table
ALTER TABLE "moderation"."reports" ADD CONSTRAINT "reports_request_check" CHECK ((request_id IS NULL) = (request_hash IS NULL)), ADD COLUMN "request_id" uuid NULL, ADD COLUMN "request_hash" bytea NULL, ADD CONSTRAINT "reports_request_key" UNIQUE ("reporter_buyer_id", "request_id");
-- Create index "reports_reporter_time" to table: "reports"
CREATE INDEX "reports_reporter_time" ON "moderation"."reports" ("reporter_buyer_id", "created_at" DESC, "id" DESC);
-- Set comment to column: "request_id" on table: "reports"
COMMENT ON COLUMN "moderation"."reports"."request_id" IS '送出時 client 帶的 request_id（AIP-155，選填）；範圍是 (檢舉人, request_id)，同一位檢舉人重送一律回這一筆，不看案件現在的狀態（A7-D28 第 3 點）';
-- Set comment to column: "request_hash" on table: "reports"
COMMENT ON COLUMN "moderation"."reports"."request_hash" IS '送出輸入（對象、類別、說明）的 SHA-256；同一個 request_id 重送時比對內容是否相同，與 request_id 同時有或同時沒有';
