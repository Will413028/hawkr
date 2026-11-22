-- Modify "days" table
ALTER TABLE "bizcalendar"."days" DROP CONSTRAINT "days_year_check", DROP CONSTRAINT "days_year_day_check", DROP COLUMN "year";
