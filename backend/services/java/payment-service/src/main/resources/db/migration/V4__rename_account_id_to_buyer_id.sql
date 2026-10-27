-- =====================================================================
-- V4：payment_transactions.account_id → buyer_id 命名債清理
-- =====================================================================
-- Phase 2 account_id 命名債稽核（Task 18）：欄位名叫 account_id，值一直是
-- 買家身份（PaymentApplicationService.createPayment 建交易時，寫入的是
-- gateway 已解析過的現任 buyer_id，並拿來與 order-service 回傳的
-- orderInfo.buyerId 比對做自付防護——值從未是「登入帳號」語意）。
-- 純 catalog-only rename（欄位型別/內容不變），不需要 backfill 分階段。
-- =====================================================================

ALTER TABLE payment_transactions RENAME COLUMN account_id TO buyer_id;
ALTER INDEX idx_payment_transactions_account_id RENAME TO idx_payment_transactions_buyer_id;

COMMENT ON COLUMN payment_transactions.buyer_id IS '買家 buyer_id（2026-08-02 由 account_id 改名，值一直是買家身份，非登入帳號）';
