-- Drop index "idx_crm_customer_tags_customer" from table: "crm_customer_tags"
DROP INDEX "idx_crm_customer_tags_customer";
-- Modify "crm_customer_tags" table (account_id already holds the seller surrogate — rename only)
ALTER TABLE "crm_customer_tags" RENAME COLUMN "account_id" TO "seller_account_id";
-- Rename auto-named UNIQUE constraint to match the renamed column
ALTER TABLE "crm_customer_tags" RENAME CONSTRAINT "crm_customer_tags_account_id_customer_account_id_tag_id_key" TO "crm_customer_tags_seller_account_id_customer_account_id_tag_key";
-- Create index "idx_crm_customer_tags_customer" to table: "crm_customer_tags"
CREATE INDEX "idx_crm_customer_tags_customer" ON "crm_customer_tags" ("seller_account_id", "customer_account_id");
-- Drop index "idx_crm_customers_customer_type" from table: "crm_customers"
DROP INDEX "idx_crm_customers_customer_type";
-- Modify "crm_customers" table (account_id already holds the seller surrogate — rename only)
ALTER TABLE "crm_customers" RENAME COLUMN "account_id" TO "seller_account_id";
-- Rename auto-named UNIQUE constraint to match the renamed column
ALTER TABLE "crm_customers" RENAME CONSTRAINT "crm_customers_account_id_customer_account_id_key" TO "crm_customers_seller_account_id_customer_account_id_key";
-- Create index "idx_crm_customers_customer_type" to table: "crm_customers"
CREATE INDEX "idx_crm_customers_customer_type" ON "crm_customers" ("seller_account_id", "customer_type");
-- Rename index "idx_crm_customers_account_id" to "idx_crm_customers_seller_account_id"
ALTER INDEX "idx_crm_customers_account_id" RENAME TO "idx_crm_customers_seller_account_id";
-- Drop index "idx_crm_tags_account_id" from table: "crm_tags"
DROP INDEX "idx_crm_tags_account_id";
-- Modify "crm_tags" table (account_id already holds the seller surrogate — rename only)
ALTER TABLE "crm_tags" RENAME COLUMN "account_id" TO "seller_account_id";
-- Create index "idx_crm_tags_account_id" to table: "crm_tags"
CREATE INDEX "idx_crm_tags_account_id" ON "crm_tags" ("seller_account_id");
