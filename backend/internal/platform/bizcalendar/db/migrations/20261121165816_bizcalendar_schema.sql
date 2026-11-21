-- Add new schema named "bizcalendar"
CREATE SCHEMA "bizcalendar";
-- Create "days" table
CREATE TABLE "bizcalendar"."days" (
  "day" date NOT NULL,
  "holiday" boolean NOT NULL,
  "note" text NOT NULL DEFAULT '',
  "year" smallint NOT NULL,
  PRIMARY KEY ("day"),
  CONSTRAINT "days_note_check" CHECK (note !~ '^[\u0009\u000a\u000b\u000c\u000d \u0085                 　]|[\u0009\u000a\u000b\u000c\u000d \u0085                 　]$'::text),
  CONSTRAINT "days_year_check" CHECK (((year >= 1) AND (year <= 9999)) AND ((year)::numeric = EXTRACT(year FROM day)))
);
-- Create index "days_by_year" to table: "days"
CREATE INDEX "days_by_year" ON "bizcalendar"."days" ("year");
