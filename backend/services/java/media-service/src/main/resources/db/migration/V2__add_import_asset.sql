-- ============================================================================
-- V2: 新增 import_asset 表（08 號設計書 §7.1）
--
-- 用途：Product Import Service 委派的外部 URL 下載生命週期管理；
--       與既有 upload_session（前端上傳）/ media_asset（已正式發布）並排
-- 詳見：docs/詳細設計書/08-多媒體微服務（Media Service）詳細設計書.md §7.1
-- ============================================================================

CREATE TABLE IF NOT EXISTS import_asset (
    id                          VARCHAR(64) PRIMARY KEY,           -- UUID（Media 自管）
    account_id                  BIGINT      NOT NULL,
    import_job_id               BIGINT      NOT NULL,
    import_item_id              BIGINT      NOT NULL,
    callback_correlation_id     BIGINT      NOT NULL,              -- Import 端 product_import_item_asset.id（callback 時 echo）
    source_url                  VARCHAR(2000) NOT NULL,
    source_url_hash             VARCHAR(64) NOT NULL,
    ref_type                    VARCHAR(40) NOT NULL,              -- PRODUCT_IMPORT_PREVIEW | PRODUCT_IMPORT_PREVIEW_DESC
    status                      VARCHAR(20) NOT NULL,              -- 9401~9405（ImportAssetMediaStatus）
    object_key                  VARCHAR(500),                      -- OSS tmp 路徑
    content_type                VARCHAR(50),
    file_size                   BIGINT,
    preview_url                 VARCHAR(1000),
    finalized_asset_id          BIGINT,                            -- FINALIZED 後對應 media_asset.id
    finalized_object_key        VARCHAR(500),
    finalized_ref_type          VARCHAR(40),                       -- PRODUCT | PRODUCT_DESC
    failure_reason              VARCHAR(80),
    failure_detail              TEXT,
    expires_at                  TIMESTAMPTZ NOT NULL,
    finalized_at                TIMESTAMPTZ,
    expired_at                  TIMESTAMPTZ,
    version                     INTEGER     NOT NULL DEFAULT 0,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uk_import_asset_dedup UNIQUE (account_id, import_job_id, source_url_hash)
);

CREATE INDEX IF NOT EXISTS idx_import_asset_status_expires
    ON import_asset (status, expires_at);

CREATE INDEX IF NOT EXISTS idx_import_asset_job
    ON import_asset (import_job_id);

CREATE INDEX IF NOT EXISTS idx_import_asset_item
    ON import_asset (import_item_id);

-- updated_at 自動更新觸發器（重用 V1 的 update_updated_at_column()）
DROP TRIGGER IF EXISTS update_import_asset_updated_at ON import_asset;
CREATE TRIGGER update_import_asset_updated_at
    BEFORE UPDATE ON import_asset
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 註解
COMMENT ON TABLE import_asset IS 'Product Import 委派下載資產：外部 URL → SafeFetch → OSS tmp → FINALIZED 搬到正式 PRODUCT/PRODUCT_DESC 路徑';
COMMENT ON COLUMN import_asset.id IS 'UUID（Media 自管）；ImportDownloadAsset 回應 / callback event 內均為此 id';
COMMENT ON COLUMN import_asset.callback_correlation_id IS 'Import 端 product_import_item_asset.id；callback event 內 echo 回 Import Service';
COMMENT ON COLUMN import_asset.source_url_hash IS 'SHA256(source_url)；UNIQUE(account_id, import_job_id, source_url_hash) 並發保護';
COMMENT ON COLUMN import_asset.status IS '9401 DOWNLOADING / 9402 UPLOADED / 9403 FINALIZED / 9404 FAILED / 9405 EXPIRED（ImportAssetMediaStatus）';
COMMENT ON COLUMN import_asset.ref_type IS 'PRODUCT_IMPORT_PREVIEW（主圖階段）/ PRODUCT_IMPORT_PREVIEW_DESC（描述圖階段）';
COMMENT ON COLUMN import_asset.finalized_ref_type IS 'FINALIZED 後對應的 media_asset.ref_type（PRODUCT / PRODUCT_DESC）';
COMMENT ON COLUMN import_asset.expires_at IS '生命週期終點（通常 ImportJob.expires_at + 1d buffer）';
