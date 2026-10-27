-- Create "seller_product_view_log" table
CREATE TABLE "seller_product_view_log" (
  "id" bigint NOT NULL,
  "seller_id" bigint NOT NULL,
  "product_id" bigint NOT NULL,
  "source" character varying(10) NOT NULL,
  "viewed_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "idx_spvl_seller_viewed" to table: "seller_product_view_log"
CREATE INDEX "idx_spvl_seller_viewed" ON "seller_product_view_log" ("seller_id", "viewed_at" DESC);
