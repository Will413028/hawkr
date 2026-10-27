-- ============================================================================
-- V3: 新增 import_asset.source_type / staged_session_id 支援 BindStagedAsset
--
-- 對應 40 號設計書 §3.4 / §4.3：
--   - source_type = 'URL'    → 外部 URL 下載路徑（source_url + source_url_hash 必填）
--   - source_type = 'STAGED' → 賣家本地上傳 staged_token（staged_session_id 必填）
--
-- V1/V2 既有 row 一律視為 URL（DEFAULT 'URL'）
-- ============================================================================

-- 1) 新增欄位
ALTER TABLE import_asset
    ADD COLUMN IF NOT EXISTS source_type        VARCHAR(20),
    ADD COLUMN IF NOT EXISTS staged_session_id  VARCHAR(64);

UPDATE import_asset SET source_type = 'URL' WHERE source_type IS NULL;

ALTER TABLE import_asset
    ALTER COLUMN source_type SET NOT NULL,
    ALTER COLUMN source_type SET DEFAULT 'URL';

-- 2) 放寬既有欄位（STAGED row 不寫 source_url / source_url_hash）
ALTER TABLE import_asset
    ALTER COLUMN source_url      DROP NOT NULL,
    ALTER COLUMN source_url_hash DROP NOT NULL;

-- 3) source_type 列舉檢查
ALTER TABLE import_asset
    DROP CONSTRAINT IF EXISTS chk_import_asset_source_type;
ALTER TABLE import_asset
    ADD CONSTRAINT chk_import_asset_source_type CHECK (source_type IN ('URL','STAGED'));

-- 4) URL / STAGED 二擇一一致性
ALTER TABLE import_asset
    DROP CONSTRAINT IF EXISTS chk_import_asset_source_payload;
ALTER TABLE import_asset
    ADD CONSTRAINT chk_import_asset_source_payload CHECK (
        (source_type = 'URL'
            AND source_url IS NOT NULL
            AND source_url_hash IS NOT NULL
            AND staged_session_id IS NULL)
        OR
        (source_type = 'STAGED'
            AND staged_session_id IS NOT NULL
            AND source_url IS NULL
            AND source_url_hash IS NULL)
    );

-- 5) 既有 dedup UNIQUE 改為條件式（只對 URL 啟用），避免 STAGED row NULL hash 撞鎖
ALTER TABLE import_asset
    DROP CONSTRAINT IF EXISTS uk_import_asset_dedup;
CREATE UNIQUE INDEX IF NOT EXISTS uk_import_asset_dedup_url
    ON import_asset (account_id, import_job_id, source_url_hash)
    WHERE source_type = 'URL';

-- 6) STAGED dedup：同 (account_id, import_job_id, staged_session_id) 唯一
CREATE UNIQUE INDEX IF NOT EXISTS uk_import_asset_dedup_staged
    ON import_asset (account_id, import_job_id, staged_session_id)
    WHERE source_type = 'STAGED';

COMMENT ON COLUMN import_asset.source_type IS 'URL=外部下載 / STAGED=賣家本地上傳 staged_token 綁定（40 號設計書 §4.3）';
COMMENT ON COLUMN import_asset.staged_session_id IS 'source_type=STAGED 時必填，對應 upload_session.id（UUID 字串）';
