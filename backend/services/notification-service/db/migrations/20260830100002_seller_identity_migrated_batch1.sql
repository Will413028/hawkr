-- Seller notification identity-migration marker, batch 1 of N (issue #2707).
--
-- `identity_migrated` (schema migration 20260830100001) 標記「這張 seller 側
-- template 的 target_user_field 現在指向的是已解析的當前 store_id，不是登入
-- account_id」——語意完全綁定 20260830100000_seller_target_field_store_id_rekey_batch1.sql
-- （commit 70054aa17，「fix(notification-service): retarget seller-side
-- templates to resolved store_id」）改的那 9 支 target_user_field。兩份 migration
-- 是同一件事的兩面：那份改「寫哪個欄位」，這份標記「讀端該用哪個身分空間核對它」。
--
-- 讀端（TestTemplateContract 的 expectedIdentitySpace、
-- scripts/audit-notification-identity.py 的 Spaces.classify）從此逐 event_type
-- 判斷：`identity_migrated = true` 期望 store_id，`false`（其餘 41 支，含 39 支
-- 待修 + 2 支不受影響）維持期望 account_id——不是一次性全部反轉。之後每修好一批
-- 41 支裡的 template（把 target_user_field 改指向已解析 store_id），在同一個
-- migration 裡把對應列的 identity_migrated 一併翻成 true 即可，兩支 gate 的程式碼
-- 都不必再改。
--
-- 9 支清單逐字取自 20260830100000 的三個 WHERE ... event_type IN (...) 子句
-- （Part A/B/C/D 合計 9 條，不是改寫或摘要）：
UPDATE "notification_templates"
SET identity_migrated = true,
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND event_type IN (
    'order.cancelled.merchant',
    'order.completed.merchant',
    'review.created',
    'product.certification.approved.seller',
    'product.certification.rejected.seller',
    'product.certification.revoked.seller',
    'seller.suspended.notice',
    'seller.unsuspended.notice',
    'delegation.operation.completed.seller'
  );
