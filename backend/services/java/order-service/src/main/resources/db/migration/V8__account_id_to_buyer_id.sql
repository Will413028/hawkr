-- buyer_id 8-service rekey, Phase 1 (order-service): buyer_account_id -> buyer_id.
-- Mirrors V7's treatment of seller_account_id -> store_id. Metadata-only rename;
-- none of these 3 columns are PK/FK/unique-index members (verified via schema
-- read 2026-07-18). idx_orders_buyer_status already carries the "buyer" name
-- (not "account_id"), so — unlike V7's idx_orders_seller_status -> idx_orders_store_status
-- rename — no index rename is needed here; Postgres keeps the index working
-- automatically across a column rename.
ALTER TABLE orders RENAME COLUMN buyer_account_id TO buyer_id;
ALTER TABLE coupon_consumption_tasks RENAME COLUMN buyer_account_id TO buyer_id;
ALTER TABLE cart_cleanup_retry_tasks RENAME COLUMN buyer_account_id TO buyer_id;
