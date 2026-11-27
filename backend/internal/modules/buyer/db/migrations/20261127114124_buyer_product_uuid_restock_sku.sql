-- Clear rows keyed by the old integer product IDs: they cannot be carried into uuid columns (nothing is live, no data is moved)
DELETE FROM "buyer"."favorites";
DELETE FROM "buyer"."visit_history";
DELETE FROM "buyer"."restock_subscriptions";
-- Modify "favorites" table
ALTER TABLE "buyer"."favorites" DROP COLUMN "product_id", ADD COLUMN "product_id" uuid NOT NULL, ADD PRIMARY KEY ("buyer_id", "product_id");
-- Drop index "restock_product" from table: "restock_subscriptions"
DROP INDEX "buyer"."restock_product";
-- Modify "restock_subscriptions" table
ALTER TABLE "buyer"."restock_subscriptions" DROP COLUMN "product_id", ADD COLUMN "product_id" uuid NOT NULL, ADD COLUMN "sku_id" uuid NOT NULL, ADD PRIMARY KEY ("buyer_id", "sku_id");
-- Create index "restock_sku" to table: "restock_subscriptions"
CREATE INDEX "restock_sku" ON "buyer"."restock_subscriptions" ("sku_id", "buyer_id");
-- Modify "visit_history" table
ALTER TABLE "buyer"."visit_history" DROP COLUMN "product_id", ADD COLUMN "product_id" uuid NOT NULL, ADD PRIMARY KEY ("buyer_id", "product_id");
