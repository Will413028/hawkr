-- Create "order_item_reviews" table
CREATE TABLE "order_item_reviews" (
  "id" bigint NOT NULL,
  "order_no" character varying(64) NOT NULL,
  "order_item_id" bigint NOT NULL,
  "buyer_account_id" bigint NOT NULL,
  "seller_account_id" bigint NOT NULL,
  "rating" smallint NOT NULL,
  "content" text NULL,
  "image_urls" text[] NOT NULL DEFAULT '{}',
  "sku_name" character varying(255) NOT NULL DEFAULT '',
  "item_image_url" text NOT NULL DEFAULT '',
  "price" bigint NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "order_item_reviews_item_unique" UNIQUE ("order_item_id"),
  CONSTRAINT "order_item_reviews_rating_check" CHECK ((rating >= 1) AND (rating <= 5))
);
-- Create index "idx_order_item_reviews_buyer" to table: "order_item_reviews"
CREATE INDEX "idx_order_item_reviews_buyer" ON "order_item_reviews" ("buyer_account_id", "created_at" DESC);
-- Create index "idx_order_item_reviews_order_no" to table: "order_item_reviews"
CREATE INDEX "idx_order_item_reviews_order_no" ON "order_item_reviews" ("order_no");
-- Create index "idx_order_item_reviews_seller" to table: "order_item_reviews"
CREATE INDEX "idx_order_item_reviews_seller" ON "order_item_reviews" ("seller_account_id", "created_at" DESC);
