-- account_id vs store_id audit — Plan C Phase 3（aftersales-service）：
-- aftersale_cases.seller_account_id / seller_cancel_requests.seller_account_id
-- 更名為 store_id — 名實相符，值本身不變（純 rename，非新語意；賣家身分改走
-- gateway resolver 解析出的 store_id surrogate，不再是登入 account_id）。
--
-- 部署機制：Flyway 於 pod 啟動執行，單一 transaction 完成兩張表各自的 column
-- rename + index rename + COMMENT。不需 backfill（既有值本身就是目前唯一的
-- store_id 候選——grandfather 規則下 store_id 首次等於 account_id）。
--
-- AfterSaleAuditLogEntity.actor_account_id 為稽核追蹤欄位（記錄「誰做的」而非
-- 「目前歸屬哪個 store」），不在本次更名範圍內。

ALTER TABLE aftersale_cases RENAME COLUMN seller_account_id TO store_id;
ALTER INDEX idx_aftersale_cases_seller RENAME TO idx_aftersale_cases_store;

COMMENT ON COLUMN aftersale_cases.store_id IS
  '售後案件所屬賣家的 store 代理鍵（store-stint surrogate，由 gateway resolver 注入）；Plan C Phase 3 更名，原 seller_account_id';

ALTER TABLE seller_cancel_requests RENAME COLUMN seller_account_id TO store_id;
ALTER INDEX idx_seller_cancel_seller RENAME TO idx_seller_cancel_store;

COMMENT ON COLUMN seller_cancel_requests.store_id IS
  '取消出貨請求所屬賣家的 store 代理鍵（store-stint surrogate，由 gateway resolver 注入）；Plan C Phase 3 更名，原 seller_account_id';
