-- =============================================================================
-- V2: product_import_item_asset 加入 source_type / staged_token 支援本地上傳路徑
--
-- 對應 40 號設計書 §3.4 / §4.3：
--   - source_type = 'URL'   → 既有外部 URL 下載路徑（source_url + source_url_hash 必填、staged_token NULL）
--   - source_type = 'STAGED' → 本地上傳路徑（staged_token 必填、source_url / source_url_hash NULL）
--
-- V1 已存資料一律視為 URL（migrate 後 source_type 預設 'URL'，可空欄位放寬）
-- =============================================================================

-- 1) 新增欄位（V1 既有 row 視為 URL，預設值補完後再加 NOT NULL）
ALTER TABLE product_import_item_asset
    ADD COLUMN source_type   VARCHAR(20),
    ADD COLUMN staged_token  VARCHAR(64);

UPDATE product_import_item_asset SET source_type = 'URL' WHERE source_type IS NULL;

ALTER TABLE product_import_item_asset
    ALTER COLUMN source_type SET NOT NULL,
    ALTER COLUMN source_type SET DEFAULT 'URL';

-- 2) 放寬既有欄位，使 STAGED row 可不填 source_url / source_url_hash
ALTER TABLE product_import_item_asset
    ALTER COLUMN source_url      DROP NOT NULL,
    ALTER COLUMN source_url_hash DROP NOT NULL;

-- 3) source_type 列舉檢查
ALTER TABLE product_import_item_asset
    ADD CONSTRAINT chk_piia_source_type CHECK (source_type IN ('URL','STAGED'));

-- 4) URL / STAGED 二擇一 一致性檢查
ALTER TABLE product_import_item_asset
    ADD CONSTRAINT chk_piia_source_payload CHECK (
        (source_type = 'URL'
            AND source_url IS NOT NULL
            AND source_url_hash IS NOT NULL
            AND staged_token IS NULL)
        OR
        (source_type = 'STAGED'
            AND staged_token IS NOT NULL
            AND source_url IS NULL
            AND source_url_hash IS NULL)
    );

-- 5) STAGED 不參與 source_url_hash dedup，但 staged_token 對 (job, token) 一致性可加 partial index
CREATE INDEX idx_piia_staged_token ON product_import_item_asset (import_job_id, staged_token)
    WHERE source_type = 'STAGED';

COMMENT ON COLUMN product_import_item_asset.source_type
    IS 'URL=外部 URL 下載 / STAGED=賣家本地上傳取得 staged_token（40 號設計書 §3.4）';
COMMENT ON COLUMN product_import_item_asset.staged_token
    IS '賣家本地上傳取得；source_type=STAGED 時必填，由 UpdateImportItem 階段 BindStagedAsset 綁定後填入 media_import_asset_id';
