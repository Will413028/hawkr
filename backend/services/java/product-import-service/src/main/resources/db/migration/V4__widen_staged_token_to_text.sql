-- ============================================================================
-- V4: product_import_item_asset.staged_token 從 VARCHAR(64) 放寬為 TEXT
--
-- 背景：
--   Round 5 QA BUG #10 — Media 端 BindStagedAsset 回傳的 staged_token 為 JWT 格式
--   （~400 字元），V2 schema 宣告 VARCHAR(64) 太小，INSERT 直接 throw
--   `value too long for type character varying(64)`，整個 STAGED 路徑 broken。
--
-- 修法：放寬為 TEXT（PostgreSQL TEXT 與 VARCHAR 在 storage 上一致，無 schema 升級成本）。
--   既有 partial index idx_piia_staged_token 改建一次（PG 改 column type 會自動 invalidate 索引，
--   需要 REINDEX 或重建；此處用 DROP + CREATE 確保乾淨）。
--
-- 對應 40 號設計書 §3.4。
-- ============================================================================

ALTER TABLE product_import_item_asset
    ALTER COLUMN staged_token TYPE TEXT;

-- 重建 partial index（PG 改 column type 不保留 expression index 的 plan，安全起見重建）
DROP INDEX IF EXISTS idx_piia_staged_token;
CREATE INDEX idx_piia_staged_token ON product_import_item_asset (import_job_id, staged_token)
    WHERE source_type = 'STAGED';

COMMENT ON COLUMN product_import_item_asset.staged_token
    IS '賣家本地上傳取得；source_type=STAGED 時必填，TEXT 容 JWT 格式（V4 由 VARCHAR(64) 放寬）；由 UpdateImportItem 階段 BindStagedAsset 綁定後填入 media_import_asset_id（40 號設計書 §3.4）';
