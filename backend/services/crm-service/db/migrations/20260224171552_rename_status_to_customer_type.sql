-- Rename "status" column to "customer_type" in "crm_customers" table
ALTER TABLE "crm_customers" RENAME COLUMN "status" TO "customer_type";
-- Drop old index
DROP INDEX IF EXISTS "idx_crm_customers_status";
-- Create index "idx_crm_customers_customer_type" to table: "crm_customers"
CREATE INDEX "idx_crm_customers_customer_type" ON "crm_customers" ("account_id", "customer_type");
-- Data migration: rename blacklist → blocklist
UPDATE "crm_customers" SET "customer_type" = 'blocklist' WHERE "customer_type" = 'blacklist';
