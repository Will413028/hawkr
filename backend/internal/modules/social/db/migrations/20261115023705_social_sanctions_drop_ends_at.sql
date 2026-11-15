-- Modify "sanctions" table
ALTER TABLE "social"."sanctions" DROP CONSTRAINT "sanctions_ends_at_check", DROP COLUMN "ends_at";
