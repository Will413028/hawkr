-- Create "site_contents" table
CREATE TABLE "site_contents" (
  "content_key" character varying(100) NOT NULL,
  "label" character varying(255) NOT NULL DEFAULT '',
  "content_text" text NOT NULL DEFAULT '',
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("content_key")
);
