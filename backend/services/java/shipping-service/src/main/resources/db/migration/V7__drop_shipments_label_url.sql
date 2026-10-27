-- 49 號設計書 §5.3 #8：shipments.label_url 為 label_object_key 的冗餘 URL 欄位。
-- DB 一律只存 objectKey，讀取端以 label_object_key 組 URL；DROP 完整 URL 欄位。
ALTER TABLE shipments DROP COLUMN IF EXISTS label_url;
