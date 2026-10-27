-- 49 號 §5.3 #3：orders 圖片欄位誠實命名（名帶 url 實存 objectKey → 改為 *_keys）。
-- RENAME COLUMN 保值（不搬資料）；存量值型態修正（URL→objectKey strip）由一次性遷移工具處理，
-- 顯示端 Gateway BuildURL 對存量完整 URL 走 passthrough，不破圖。
ALTER TABLE orders RENAME COLUMN shipment_photo_urls TO shipment_photo_keys;
ALTER TABLE orders RENAME COLUMN receipt_photo_urls TO receipt_photo_keys;

-- orders.review JSONB 內層 key 誠實化：imageUrls → imageKeys（保值，僅改 key 名，供新版 ReviewSnapshot 反序列化）。
UPDATE orders
SET review = (review - 'imageUrls') || jsonb_build_object('imageKeys', review -> 'imageUrls')
WHERE review IS NOT NULL AND review ? 'imageUrls';
