-- Modify "kyc_applications" table
ALTER TABLE "seller"."kyc_applications" DROP CONSTRAINT "kyc_applications_step_check", DROP COLUMN "current_step";
