-- Drop CRM and Marketing tables from seller-service
-- These tables have been migrated to crm-service (hawkr_crm database)

DROP TABLE IF EXISTS "crm_customer_tags";
DROP TABLE IF EXISTS "crm_customers";
DROP TABLE IF EXISTS "crm_tags";
DROP TABLE IF EXISTS "marketing_tasks";
