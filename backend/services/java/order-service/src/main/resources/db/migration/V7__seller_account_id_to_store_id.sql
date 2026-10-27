-- Plan C Phase 2 (2026-07-15 design, coupon/order/aftersales store_id rekey):
-- Step 1/2: rename the pre-existing, semantically-unrelated CVS 超商門市 columns out of the
-- way FIRST. orders.store_id / orders.store_name are the convenience-store pickup code/name
-- (populated only when shipping_method=3002) — completely unrelated to seller identity.
-- Renaming seller_account_id -> store_id below would otherwise collide with these.
ALTER TABLE orders RENAME COLUMN store_id TO cvs_store_id;
ALTER TABLE orders RENAME COLUMN store_name TO cvs_store_name;

-- Step 2: the actual rekey — seller identity column now keys on the store surrogate,
-- matching the ② store-rekey epic's convention (seller-service/product-service/cart-service).
ALTER TABLE orders RENAME COLUMN seller_account_id TO store_id;
ALTER INDEX idx_orders_seller_status RENAME TO idx_orders_store_status;
