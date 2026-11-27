-- 訂單明細的商品與 SKU ID 改成 catalog 的 UUID（UUIDv7），不保留數字舊 ID 的對照。
-- 數字的舊 ID 指向即將刪除的 product-service，沒有可轉換的對應，也沒有上線的資料。
--
-- 庫存交易紀錄的 payload_snapshot 內是數字 skuId，轉型後無法歸還庫存；待付款訂單的取消與補償都依賴它，
-- 所以連同庫存交易紀錄與待付款訂單（及其明細）一併清除。
DELETE FROM stock_tx_records;
DELETE FROM order_items WHERE order_no IN (SELECT order_no FROM orders WHERE status = '0');
DELETE FROM orders WHERE status = '0';

-- 其餘訂單的數字舊值轉成 NULL（兩欄都可為 NULL：product_id 本來就可空，sku_id 由 V17 放寬；
-- NULL 的語意是沒有平台 SKU 的明細，訂單與金額快照不動）。
-- uk_order_items_order_sku (order_no, sku_id) 與 idx_order_items_sku_id 隨欄位型別重建，名稱與定義不變。
ALTER TABLE order_items ALTER COLUMN product_id TYPE UUID USING NULL;
ALTER TABLE order_items ALTER COLUMN sku_id TYPE UUID USING NULL;
