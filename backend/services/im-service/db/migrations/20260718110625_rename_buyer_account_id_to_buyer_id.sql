-- Modify "conversations" table (buyer_account_id already holds the stint-scoped buyer_id — rename only, mirrors the seller_account_id -> store_id rename on this same table, 20260711163453)
ALTER TABLE "conversations" RENAME COLUMN "buyer_account_id" TO "buyer_id";
