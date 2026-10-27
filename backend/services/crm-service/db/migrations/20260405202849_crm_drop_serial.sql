-- Modify "crm_customers" table
ALTER TABLE "crm_customers" ALTER COLUMN "id" DROP DEFAULT;
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "crm_customers_id_seq";
-- Modify "crm_tags" table
ALTER TABLE "crm_tags" ALTER COLUMN "id" DROP DEFAULT;
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "crm_tags_id_seq";
