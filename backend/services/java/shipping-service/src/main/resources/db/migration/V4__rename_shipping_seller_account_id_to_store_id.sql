-- D7 Stage 2（design 2026-07-12-store-id-rename-stage2-design.md §2 shipping.proto row +
-- this repo's PR-2/shipping-service plan）：本 service 3 個 seller_account_id 欄位全數
-- 更名為 store_id — 名實相符，值本身不變（純 rename，非新語意）。
--
-- ⚠️ shipments 表本身有名稱碰撞，本 migration 用「兩步走」解決（2026-07-12 Will 同日
-- 撤回「shipments.seller_account_id 永久排除」的第一版決定，改採此法）：
--   1) shipments 已存在一個完全不相關、語意不同的既有欄位 store_id（VARCHAR(20)，
--      超商取貨門市代碼，如 "916712"，V1 建立）。直接 RENAME seller_account_id 為
--      store_id 會撞名（column "store_id" of relation "shipments" already exists）。
--      解法：先把既有 store_id 讓名，改名為 cvs_store_code（沿用本 service 既有的
--      「XXX_store_code」命名慣例，見 buyer_store_pickups.store_code、
--      seller_shipment_settings.return_store_code，同一概念的既有先例）。
--   2) 名稱讓出後，seller_account_id 才能安全 RENAME 為 store_id，與本次全 D7 效力
--      範圍內「代理鍵一律叫 store_id」的規則一致，不再有任何永久例外。
-- 執行順序不可顛倒：第 1 步必須先跑完，第 2 步的目標名稱 store_id 才會是空的。
--
-- 部署機制：Flyway 於 pod 啟動執行，單一 transaction 完成三張表各自的 column rename +
-- index rename + COMMENT。不需 backfill（值本身已正確，這裡只是改名）。

-- ---- shipments：兩步走解碰撞 ----
ALTER TABLE shipments RENAME COLUMN store_id TO cvs_store_code;
ALTER TABLE shipments RENAME COLUMN seller_account_id TO store_id;
ALTER INDEX idx_shipments_seller RENAME TO idx_shipments_store;

COMMENT ON COLUMN shipments.store_id IS
  '物流單所屬賣家的 store 代理鍵（store-stint surrogate，由 gateway resolver 注入）；D7 Stage 2 更名，原 seller_account_id';
COMMENT ON COLUMN shipments.cvs_store_code IS
  '超商取貨門市代碼（PayUni 門市地圖回傳，如 "916712"）；與上方 store_id 代理鍵無關，僅巧合曾經同名。D7 Stage 2 讓名，原 store_id';

-- ---- seller_shipment_settings / seller_shipment_onboarding_logs：一律無碰撞 ----
ALTER TABLE seller_shipment_settings RENAME COLUMN seller_account_id TO store_id;
ALTER INDEX idx_settings_seller_type RENAME TO idx_settings_store_type;
ALTER INDEX idx_settings_seller RENAME TO idx_settings_store;

COMMENT ON COLUMN seller_shipment_settings.store_id IS
  '建店設定所屬賣家的 store 代理鍵（store-stint surrogate，由 gateway resolver 注入）；D7 Stage 2 更名，原 seller_account_id';

ALTER TABLE seller_shipment_onboarding_logs RENAME COLUMN seller_account_id TO store_id;
ALTER INDEX idx_onboarding_logs_seller RENAME TO idx_onboarding_logs_store;

COMMENT ON COLUMN seller_shipment_onboarding_logs.store_id IS
  '建店操作日誌所屬賣家的 store 代理鍵（store-stint surrogate，由 gateway resolver 注入）；D7 Stage 2 更名，原 seller_account_id';
