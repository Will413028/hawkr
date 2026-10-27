-- Modify "crm_customers" table
ALTER TABLE "crm_customers" ADD COLUMN "nickname" character varying(100) NOT NULL DEFAULT '', ADD COLUMN "phone" character varying(20) NOT NULL DEFAULT '';
