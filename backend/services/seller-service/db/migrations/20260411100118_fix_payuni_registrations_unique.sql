-- Modify "certification_applications" table
ALTER TABLE "certification_applications" ALTER COLUMN "id" DROP DEFAULT;
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "certification_applications_id_seq";
-- Modify "payuni_registrations" table
ALTER TABLE "payuni_registrations" DROP CONSTRAINT "payuni_registrations_application_id_key", ALTER COLUMN "id" DROP DEFAULT, ADD CONSTRAINT "payuni_registrations_account_id_key" UNIQUE ("account_id");
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "payuni_registrations_id_seq";
