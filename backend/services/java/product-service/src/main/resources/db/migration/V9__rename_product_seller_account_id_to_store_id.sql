-- D7 Stage 2（design 2026-07-12-store-id-rename-stage2-design.md §2 PR-B）：product 商品擁有權
-- key 從 seller_account_id 更名為 store_id — 名實相符，值本身不變（純 rename，非新語意）。
--
-- 部署機制：Flyway 於 pod 啟動執行，單一 transaction 完成 column rename + 2 個索引 rename +
-- COMMENT 更新。不需 backfill（V8 的值已經是正確的 store 代理鍵，這裡只是改名）。
ALTER TABLE product RENAME COLUMN seller_account_id TO store_id;

-- 鏡射 V8 建立的兩個索引名稱（idx_product_seller_account_id / idx_product_status_seller_account_id）
ALTER INDEX idx_product_seller_account_id RENAME TO idx_product_store_id;
ALTER INDEX idx_product_status_seller_account_id RENAME TO idx_product_status_store_id;

COMMENT ON COLUMN product.store_id IS
  '商品擁有權 key（store 代理鍵，非登入 account_id）；建立當下由請求 accountId snapshot（resolver 上線後為 current surrogate），永不改寫（entity updateStrategy=NEVER）';
