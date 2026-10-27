-- Modify "certification_applications" table
ALTER TABLE "certification_applications" ADD COLUMN "store_description" text NULL;
-- Modify "sellers" table
ALTER TABLE "sellers" DROP COLUMN "address", ADD COLUMN "city" character varying(50) NULL, ADD COLUMN "district" character varying(50) NULL, ADD COLUMN "address_detail" character varying(255) NULL, ADD COLUMN "postal_code" character varying(10) NULL;
