-- ============================================================================
-- V5: import_asset.account_id -> store_id 命名債正名
-- (account_id naming rename campaign Phase 2, Task 20)
--
-- import_asset.account_id 自 V2 建表起存的自始至終都是賣家 store_id
-- (ImportDownloadAsset / BindStagedAsset 收到 product-import-service 傳入的
-- store_id 後直接落庫，見 ImportAssetService.importDownloadAsset / bindStagedAsset)，
-- 從未是登入 account_id。對照組 upload_session.account_id / media_asset.account_id
-- 是真正的登入 principal(或系統上傳 sentinel 0)，維持不動、不在本次範圍。
--
-- 無關聯 index/constraint 名稱含 "account"(uk_import_asset_dedup_url /
-- uk_import_asset_dedup_staged / idx_import_asset_job / idx_import_asset_item /
-- idx_import_asset_status_expires 皆已是中性命名)，故只需 RENAME COLUMN +
-- 更新引用該欄位的 COMMENT 文字，PostgreSQL 會自動讓既有的 UNIQUE INDEX 定義
-- 跟著欄位改名(index 內部以 attnum 追蹤欄位，不需要重建)。
-- ============================================================================

ALTER TABLE import_asset RENAME COLUMN account_id TO store_id;

COMMENT ON COLUMN import_asset.store_id IS '下載/綁定當下的賣家 store_id（product-import-service 傳入）；Phase 2 Task 20 由 account_id 更名，值未變';
COMMENT ON COLUMN import_asset.source_url_hash IS 'SHA256(source_url)；UNIQUE(store_id, import_job_id, source_url_hash) 並發保護';
