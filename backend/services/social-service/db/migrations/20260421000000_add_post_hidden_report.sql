-- Modify "cms_post" table
ALTER TABLE "cms_post" ADD COLUMN "hidden_at" timestamptz NULL, ADD COLUMN "hidden_reason" text NULL;
-- Create "cms_post_report" table
CREATE TABLE "cms_post_report" (
  "id" bigint NOT NULL,
  "post_id" bigint NOT NULL,
  "reporter_id" bigint NOT NULL,
  "reason_code" character varying(30) NOT NULL,
  "description" text NULL,
  "create_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  UNIQUE ("post_id", "reporter_id")
);
-- Create index "idx_cms_post_report_post" to table: "cms_post_report"
CREATE INDEX "idx_cms_post_report_post" ON "cms_post_report" ("post_id", "create_time" DESC);
