-- Create index "cases_by_time" to table: "cases"
CREATE INDEX "cases_by_time" ON "moderation"."cases" ("opened_at" DESC, "id" DESC);
