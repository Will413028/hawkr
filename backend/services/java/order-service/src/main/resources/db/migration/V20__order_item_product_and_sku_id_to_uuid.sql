-- B4-4a（D27）：訂單明細的商品與 SKU ID 改成 catalog 的 UUID（UUIDv7），不保留數字舊 ID 的對照。
-- 數字的舊 ID 指向即將刪除的 product-service，沒有可轉換的對應，也沒有上線的資料：既有的數字值轉成 NULL（兩欄都可為 NULL：
-- product_id 本來就可空，sku_id 由 V17 放寬；NULL 的語意是沒有平台 SKU 的明細，訂單與金額快照不動）。
-- uk_order_items_order_sku (order_no, sku_id) 與 idx_order_items_sku_id 隨欄位型別重建，名稱與定義不變。
ALTER TABLE order_items ALTER COLUMN product_id TYPE UUID USING NULL;
ALTER TABLE order_items ALTER COLUMN sku_id TYPE UUID USING NULL;
