-- V5: 為 MediaCallbackListener fan-out 查詢新增 index
-- issue #1030：Media 端 (account, job, sourceUrlHash) dedup 同 URL 只 1 個 import_asset，
-- callback 只回給第一個 caller；ProductImport 端多個 product_import_item_asset row 共用同一
-- media_import_asset_id，必須 fan-out UPDATE 所有 DOWNLOADING row 才能避免 stuck。
--
-- 對應 40 號設計書 §5.4.2 / §5.2 step 6。

CREATE INDEX IF NOT EXISTS idx_piia_media_dedup_callback
    ON product_import_item_asset (import_job_id, media_import_asset_id)
    WHERE status = '9302';

COMMENT ON INDEX idx_piia_media_dedup_callback IS
    'MediaCallbackListener fan-out 查詢專用（40 號設計書 §5.4.2）：以 (job, media_import_asset_id) 找全部 DOWNLOADING(9302) row 批次 UPDATE，解 Media 端 dedup 同 URL 後 callback 漏發其他 row 問題。';
