-- Create "seller_store_visitors" table
CREATE TABLE "seller_store_visitors" (
  "seller_account_id"  bigint NOT NULL,
  "visitor_account_id" bigint NOT NULL,
  "visit_count"        integer NOT NULL DEFAULT 1,
  "last_visited_at"    timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("seller_account_id", "visitor_account_id")
);
-- Create index
CREATE INDEX "idx_seller_store_visitors_seller" ON "seller_store_visitors" ("seller_account_id", "last_visited_at" DESC);
