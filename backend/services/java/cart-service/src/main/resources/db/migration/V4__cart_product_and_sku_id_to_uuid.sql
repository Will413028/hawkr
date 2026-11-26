-- B4-4a（D27）：商品與 SKU 的 ID 改成 catalog 的 UUID（UUIDv7），不保留數字舊 ID 的對照。
-- 數字的 sku_id、product_id 指向即將刪除的 product-service，沒有可轉換的對應，也沒有上線的資料，所以先清掉既有的明細再換欄位型別。
-- sku_id 是兩個 partial unique index 的成員，換欄位時隨欄位一起被刪，這裡重建（名稱同 V1、V3）。
DELETE FROM cart_items;

ALTER TABLE cart_items DROP COLUMN sku_id;
ALTER TABLE cart_items DROP COLUMN product_id;
ALTER TABLE cart_items ADD COLUMN sku_id UUID NOT NULL;
ALTER TABLE cart_items ADD COLUMN product_id UUID NOT NULL;

COMMENT ON COLUMN cart_items.sku_id IS '商品 SKU ID（catalog 的 UUID）';
COMMENT ON COLUMN cart_items.product_id IS '商品 ID（catalog 的 UUID；AddItem 時從 ProductSimpleInfo 取得）';

CREATE UNIQUE INDEX uk_cart_buyer_sku ON cart_items (buyer_id, sku_id) WHERE buyer_id IS NOT NULL;
CREATE UNIQUE INDEX uk_cart_guest_sku ON cart_items (guest_id, sku_id) WHERE guest_id IS NOT NULL;
