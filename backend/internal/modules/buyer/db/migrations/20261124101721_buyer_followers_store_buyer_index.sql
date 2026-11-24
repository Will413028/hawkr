-- Drop index "subscriptions_store" from table: "seller_subscriptions"
DROP INDEX "buyer"."subscriptions_store";
-- Create index "subscriptions_store" to table: "seller_subscriptions"
CREATE INDEX "subscriptions_store" ON "buyer"."seller_subscriptions" ("store_id", "buyer_id");
