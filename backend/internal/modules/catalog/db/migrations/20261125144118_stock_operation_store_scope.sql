-- Modify "stock_operations" table
ALTER TABLE "catalog"."stock_operations" DROP CONSTRAINT "stock_operations_idempotency_key_key", ADD CONSTRAINT "stock_operations_store_check" CHECK ((kind = 'ADJUST'::text) = (store_id IS NOT NULL)), ADD COLUMN "store_id" bigint NULL;
-- Create index "stock_operations_global_key" to table: "stock_operations"
CREATE UNIQUE INDEX "stock_operations_global_key" ON "catalog"."stock_operations" ("idempotency_key") WHERE (store_id IS NULL);
-- Create index "stock_operations_store_key" to table: "stock_operations"
CREATE UNIQUE INDEX "stock_operations_store_key" ON "catalog"."stock_operations" ("store_id", "idempotency_key") WHERE (store_id IS NOT NULL);
