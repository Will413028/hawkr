-- Modify "sellers" table
ALTER TABLE "sellers" ADD COLUMN "overall_certification_status" character varying(20) NOT NULL DEFAULT 'not_started';
