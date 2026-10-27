-- Modify "conversations" table (seller_account_id already holds the store aggregate id — rename only)
ALTER TABLE "conversations" RENAME COLUMN "seller_account_id" TO "store_id";
