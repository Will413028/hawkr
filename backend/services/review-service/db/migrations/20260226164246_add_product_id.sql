-- Modify "order_item_reviews" table
ALTER TABLE "order_item_reviews" ADD COLUMN "product_id" bigint NOT NULL DEFAULT 0;
-- Create index "idx_order_item_reviews_product" to table: "order_item_reviews"
CREATE INDEX "idx_order_item_reviews_product" ON "order_item_reviews" ("product_id", "created_at" DESC);
