-- account_id vs store_id audit（docs/superpowers/plans/2026-07-13-account-id-vs-store-id-audit.md）
-- 最後一項：cart_items 賣家分組 key 從 seller_account_id 更名為 store_id — 名實相符，
-- 值本身不變（純 rename，非新語意；CartService.addItem 早已從 product-service 的
-- store_id 賦值，見 CartService.java:114）。
--
-- 部署機制：Flyway 於 pod 啟動執行，單一 transaction 完成 column rename + 索引 rename +
-- COMMENT 更新。不需 backfill。
ALTER TABLE cart_items RENAME COLUMN seller_account_id TO store_id;

-- 鏡射 V1 建立的索引名稱（idx_cart_account_seller）
ALTER INDEX idx_cart_account_seller RENAME TO idx_cart_account_store;

COMMENT ON COLUMN cart_items.store_id IS
  '賣家分組 key（store 代理鍵，非登入 account_id）；由 Product Service 回傳的 store_id 賦值（CartService.addItem），純 rename 非新語意';
