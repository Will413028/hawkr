-- ============================================================================
-- V6: media_asset.account_id 拆欄（account_id naming rename campaign Phase 3, Task 6）
--
-- media_asset.account_id 依上傳路徑帶 3 種不同語意（2026-08-02 Task 20 稽核確認）：
--   bucket 1（真實登入 account_id，~61.4%）：MediaAssetService.createMediaAsset
--   （一般買/賣家/後台上傳，session.getAccountId()）
--   bucket 2（sentinel 0，~36.7%）：MediaAssetService.serverUploadAndFinalize
--   （後端系統直傳，語意單一無歧義，不需新欄位）
--   bucket 3（賣家 store_id，~2.0%）：ImportAssetService.commitFinalize
--   （CSV/URL 商品匯入轉正，entity.getStoreId()）
--
-- bucket 1 vs bucket 3 無同列可靠判別欄位，只能靠 import_asset.finalized_asset_id
-- 反向 join 判斷「這筆 media_asset 是不是匯入管線轉正出來的」；bucket 2 用
-- account_id = 0 sentinel 即可唯一識別，故不需新欄位、拆欄後两個新欄位皆留 NULL。
-- ============================================================================

ALTER TABLE media_asset
  ADD COLUMN uploader_account_id BIGINT,
  ADD COLUMN import_store_id BIGINT;
  -- bucket 2（系統直傳）不需要新欄位：account_id=0 早已無歧義地代表「無身份」，
  -- 這類列的兩個新欄位維持 NULL。

UPDATE media_asset ma
SET import_store_id = ma.account_id
WHERE EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = ma.id);

UPDATE media_asset ma
SET uploader_account_id = ma.account_id
WHERE ma.account_id != 0
  AND NOT EXISTS (SELECT 1 FROM import_asset ia WHERE ia.finalized_asset_id = ma.id);

ALTER TABLE media_asset DROP COLUMN account_id;
-- 注意：DROP COLUMN 會自動一併移除只依附此欄位的 idx_media_asset_account
-- （V1 建立），PostgreSQL 對 plain index 不需要 CASCADE 即可連動刪除。

COMMENT ON COLUMN media_asset.uploader_account_id IS 'Real login account_id of the uploader, populated for normal buyer/seller/admin uploads via createMediaAsset. NULL for system-generated (serverUploadAndFinalize) or import-committed rows.';
COMMENT ON COLUMN media_asset.import_store_id IS 'Seller store_id, populated only for rows committed via the CSV/URL product-import pipeline (ImportAssetService.commitFinalize). NULL otherwise.';
