-- Create "store_blocks" table
CREATE TABLE "seller"."store_blocks" (
  "store_id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id", "buyer_id"),
  CONSTRAINT "store_blocks_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "store_blocks_buyer_id_check" CHECK (buyer_id > 0)
);
-- Create index "store_blocks_buyer" to table: "store_blocks"
CREATE INDEX "store_blocks_buyer" ON "seller"."store_blocks" ("buyer_id", "store_id");
