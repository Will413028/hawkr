-- Create index "reports_store_reporter_time" to table: "reports"
CREATE INDEX "reports_store_reporter_time" ON "moderation"."reports" ("reporter_store_id", "created_at" DESC, "id" DESC) WHERE (reporter_store_id IS NOT NULL);
