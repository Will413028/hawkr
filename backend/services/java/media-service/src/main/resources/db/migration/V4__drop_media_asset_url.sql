-- 49 號設計書 §5.1/§5.3 #9：media_asset.url 為冗餘欄位（url 由讀取端以 objectKey 組裝）。
-- DB 一律只存 objectKey，DROP 完整 URL 欄位。存量值改寫由 Phase 7 fix-db 處理，本檔僅 schema 演進。
ALTER TABLE media_asset DROP COLUMN IF EXISTS url;
