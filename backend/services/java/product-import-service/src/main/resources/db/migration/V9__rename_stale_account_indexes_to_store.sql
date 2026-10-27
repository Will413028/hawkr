-- Phase 2 account_id 命名債稽核（Task 17）：V8（2026-07-15）把
-- product_import_job.account_id / product_import_item.account_id 兩個欄位改名為
-- store_id，但當時漏改關聯的 4 個 index 名稱——Postgres 的 RENAME COLUMN 不會
-- 自動跟著改 index 名，這 4 個索引名稱一直殘留 "account" 至今。純改名，值與索引
-- 定義（欄位、順序、WHERE 條件）完全不變，不需要 backfill。
--
-- 索引本身不是任何 ORM mapping / SQL 文字查詢引用的對象（Postgres 不支援
-- index hint），對應用程式碼零影響，純 DBA 層命名一致性修正。

ALTER INDEX idx_pij_account_status          RENAME TO idx_pij_store_status;
ALTER INDEX idx_pij_account_hash            RENAME TO idx_pij_store_hash;
ALTER INDEX idx_pij_account_filename_active RENAME TO idx_pij_store_filename_active;
ALTER INDEX idx_pii_account_created         RENAME TO idx_pii_store_created;

COMMENT ON COLUMN product_import_job.source_file_hash IS 'SHA256(file_content) hex；24h 內同 store_id + hash 觸發 dedup';
