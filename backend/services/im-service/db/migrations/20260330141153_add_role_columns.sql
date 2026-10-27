-- Modify "conversations" table
ALTER TABLE "conversations" ADD COLUMN "buyer_account_id" bigint NOT NULL DEFAULT 0, ADD COLUMN "seller_account_id" bigint NOT NULL DEFAULT 0;
-- Backfill from existing room_id format: "b{buyerID}_s{sellerID}"
UPDATE conversations SET
  buyer_account_id = split_part(replace(room_id, 'b', ''), '_s', 1)::bigint,
  seller_account_id = split_part(replace(room_id, 'b', ''), '_s', 2)::bigint
WHERE buyer_account_id = 0;
-- Create index "idx_conversations_buyer" to table: "conversations"
CREATE INDEX "idx_conversations_buyer" ON "conversations" ("buyer_account_id");
-- Create index "idx_conversations_seller" to table: "conversations"
CREATE INDEX "idx_conversations_seller" ON "conversations" ("seller_account_id");
