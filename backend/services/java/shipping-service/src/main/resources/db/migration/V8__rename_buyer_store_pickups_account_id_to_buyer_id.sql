-- V8: buyer_store_pickups 買家常用門市改綁 buyer_id（宣告式綁定，取代零機制現狀）。
-- 買家軟刪除復活後拿到新 buyer_id，舊常用門市因 buyer_id 不匹配結構性不可見。
-- 目前 dev/prod 0 帳號分岔過，account_id 數值本身即等於應有的 buyer_id，純 rename
-- 無需 backfill（比照 backend-payuni Go 服務 buyer_id rekey Phase 1-9 的驗證結論）。
ALTER TABLE buyer_store_pickups RENAME COLUMN account_id TO buyer_id;
ALTER INDEX idx_bsp_account RENAME TO idx_bsp_buyer;
ALTER INDEX idx_bsp_account_default RENAME TO idx_bsp_buyer_default;
