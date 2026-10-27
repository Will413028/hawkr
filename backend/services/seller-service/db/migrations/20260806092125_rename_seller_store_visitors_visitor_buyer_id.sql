-- Modify "seller_store_visitors" table
-- atlas migrate diff generated DROP COLUMN + ADD COLUMN (data loss); hand-authored
-- as RENAME COLUMN instead — same net schema, keeps existing visit history and PK
-- intact (RENAME COLUMN carries constraints/indexes with it automatically).
ALTER TABLE "seller_store_visitors" RENAME COLUMN "visitor_account_id" TO "visitor_buyer_id";
