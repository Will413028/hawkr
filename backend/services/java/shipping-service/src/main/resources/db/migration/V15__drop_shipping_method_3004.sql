-- V15：配送方式值域移除 3004（7-11 大宗寄倉 B2C）
--
-- 3004 沒有對應的功能（無 adapter、無端點、無 caller），ShippingMethod 已不含 3004。
-- 值域收斂為 3001／3002／3003／3005；3005（商家自行配送）由 RFQ 訂單使用，保留。
--
-- ⚠️ DROP 與 ADD CONSTRAINT 必須同檔（Flyway 預設單檔單交易），否則兩句之間
--    有一個「完全不受約束」的窗口。
ALTER TABLE shipments DROP CONSTRAINT chk_shipping_method;
ALTER TABLE shipments ADD  CONSTRAINT chk_shipping_method
    CHECK (shipping_method IN ('3001', '3002', '3003', '3005'));
