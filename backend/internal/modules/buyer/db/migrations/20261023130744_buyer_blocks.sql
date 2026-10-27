-- Create "blocked_buyers" table
CREATE TABLE "buyer"."blocked_buyers" (
  "blocker_buyer_id" bigint NOT NULL,
  "blocked_buyer_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("blocker_buyer_id", "blocked_buyer_id"),
  CONSTRAINT "blocked_buyers_blocked_buyer_id_fkey" FOREIGN KEY ("blocked_buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "blocked_buyers_blocker_buyer_id_fkey" FOREIGN KEY ("blocker_buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "blocked_buyers_not_self_check" CHECK (blocker_buyer_id <> blocked_buyer_id)
);
-- Create index "blocked_buyers_blocked" to table: "blocked_buyers"
CREATE INDEX "blocked_buyers_blocked" ON "buyer"."blocked_buyers" ("blocked_buyer_id", "blocker_buyer_id");
-- Create "blocked_stores" table
CREATE TABLE "buyer"."blocked_stores" (
  "blocker_buyer_id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("blocker_buyer_id", "store_id"),
  CONSTRAINT "blocked_stores_blocker_buyer_id_fkey" FOREIGN KEY ("blocker_buyer_id") REFERENCES "buyer"."buyers" ("buyer_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "blocked_stores_store_id_check" CHECK (store_id > 0)
);
-- Create index "blocked_stores_store" to table: "blocked_stores"
CREATE INDEX "blocked_stores_store" ON "buyer"."blocked_stores" ("store_id", "blocker_buyer_id");
