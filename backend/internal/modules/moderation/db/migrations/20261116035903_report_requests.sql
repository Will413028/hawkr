-- Create index "reports_reporter_time" to table: "reports"
CREATE INDEX "reports_reporter_time" ON "moderation"."reports" ("reporter_buyer_id", "created_at" DESC, "id" DESC);
-- Create "report_requests" table
CREATE TABLE "moderation"."report_requests" (
  "reporter_buyer_id" bigint NOT NULL,
  "request_id" uuid NOT NULL,
  "report_id" uuid NOT NULL,
  "request_hash" bytea NOT NULL,
  PRIMARY KEY ("reporter_buyer_id", "request_id"),
  CONSTRAINT "report_requests_report_id_fkey" FOREIGN KEY ("report_id") REFERENCES "moderation"."reports" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "report_requests_hash_check" CHECK (octet_length(request_hash) = 32),
  CONSTRAINT "report_requests_reporter_check" CHECK (reporter_buyer_id > 0)
);
-- Create index "report_requests_report" to table: "report_requests"
CREATE INDEX "report_requests_report" ON "moderation"."report_requests" ("report_id");
