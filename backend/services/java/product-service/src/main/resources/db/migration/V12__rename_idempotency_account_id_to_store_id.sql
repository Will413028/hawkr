-- ============================================================================
-- V12: idempotency_request.account_id → store_id（account_id 命名債正名，Phase 2 Task 16）
-- 對應設計書：2026-08-02-account-id-naming-rename-phase2-design.md
-- 背景：idempotency_request.account_id 欄位值語意其實是賣家 store 代理鍵——唯二兩個
--       寫入呼叫端（ProductService.createProduct / updateProduct）皆傳入
--       CreateProductDTO/UpdateProductDTO.storeId（proto CreateProductRequest/
--       UpdateProductRequest 本身早就叫 store_id，只有這張表跟 Java 識別字沒跟上）。
--       純 rename，值本身不變。
-- ============================================================================
ALTER TABLE idempotency_request RENAME COLUMN account_id TO store_id;

ALTER INDEX idx_idempotency_account_id RENAME TO idx_idempotency_store_id;

COMMENT ON COLUMN idempotency_request.store_id IS
  '賣家 store 代理鍵（account_id 命名債正名，Phase 2 Task 16；值語意本來就是 CreateProduct/UpdateProduct 呼叫端的 store_id，唯一兩個寫入路徑皆為賣家操作)';
