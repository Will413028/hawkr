-- Create "store_buyer_membership" table
CREATE TABLE "coupon"."store_buyer_membership" (
  "store_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id", "buyer_id"),
  CONSTRAINT "membership_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "membership_store_check" CHECK (store_id > 0)
);
-- Create index "membership_by_buyer" to table: "store_buyer_membership"
CREATE INDEX "membership_by_buyer" ON "coupon"."store_buyer_membership" ("buyer_id");
