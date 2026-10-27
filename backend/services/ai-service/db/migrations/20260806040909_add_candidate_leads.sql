-- Create "candidate_leads" table
CREATE TABLE "candidate_leads" (
  "id" bigint NOT NULL,
  "source_sheet" text NOT NULL,
  "category" text NOT NULL DEFAULT '',
  "product_summary" text NOT NULL DEFAULT '',
  "region" text NOT NULL DEFAULT '',
  "positioning" text NOT NULL DEFAULT '',
  "research_summary" text NOT NULL DEFAULT '',
  "public_website" text NOT NULL DEFAULT '',
  "public_email" text NOT NULL DEFAULT '',
  "public_phone" text NOT NULL DEFAULT '',
  "public_social" text NOT NULL DEFAULT '',
  "priority_letter" text NOT NULL DEFAULT '',
  "priority_stars" integer NOT NULL DEFAULT 0,
  "imported_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_candidate_leads_category" to table: "candidate_leads"
CREATE INDEX "idx_candidate_leads_category" ON "candidate_leads" ("category");
