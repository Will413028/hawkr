-- SKU ID 改成 catalog 的 UUID（UUIDv7），不保留數字舊 ID 的對照。
-- 售後明細的 sku_id 是從訂單明細複製的快照，數字的舊 ID 指向即將刪除的 product-service，沒有可轉換的對應，也沒有上線的資料：
-- 既有的數字值轉成 NULL（V10 已放寬為可為 NULL，語意是沒有平台 SKU 的明細）。
ALTER TABLE aftersale_case_items ALTER COLUMN sku_id TYPE UUID USING NULL;
