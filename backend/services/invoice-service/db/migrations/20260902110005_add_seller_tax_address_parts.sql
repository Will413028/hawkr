-- Modify "seller_tax_profiles" table
ALTER TABLE "seller_tax_profiles" ADD COLUMN "address_city" character varying(32) NULL, ADD COLUMN "address_district" character varying(32) NULL;
