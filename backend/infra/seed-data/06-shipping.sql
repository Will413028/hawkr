-- ============================================================================
-- Hawkr 測試資料：配送設定 (hawkr_shipping)
-- ============================================================================
-- 賣家配送設定：store_id 1009（store_code 9，mer_id HAWK9）、store_id 1004（store_code 4，mer_id HAWK4）；mer_id 是 HAWK 加 store_code，
-- 店家在單體的 seller schema，由 backend 的 make dev-seed 寫入
-- shipping_type: CodeMeta ShippingType（3651=CVS_C2C 超商店到店 / 3652=HOME_DELIVERY 宅配到府；
--   見 docs/代碼管理（CodeMeta）/業務類型碼.md）
-- 欄位名為 store_id（非 seller_account_id；schema 已於 shipping-service migration 完成 rename）
-- onboarding_status: value（3602=ACTIVE）
-- ============================================================================

BEGIN;

-- 清除舊測試資料
DELETE FROM seller_shipment_settings WHERE store_id IN (1009, 1004);

-- 店家 1009（翠綠有機農場）- 黑貓宅急便 + 7-11 店到店
INSERT INTO seller_shipment_settings (
  id, store_id, shipping_type, mer_id,
  onboarding_status, enabled, goods_type,
  sender_name, sender_phone, sender_city, sender_district, sender_address_detail, sender_postal_code,
  return_name, return_phone, return_city, return_district, return_address_detail, return_postal_code
) VALUES
  -- 宅配
  (10001, 1009, '3652', 'HAWK9',
   '3602', true, NULL,
   '翠綠農場', '0912345678', '台南市', '新化區', '中山路100號', '712',
   '翠綠農場', '0912345678', '台南市', '新化區', '中山路100號', '712'),
  -- 超商
  (10002, 1009, '3651', 'HAWK9',
   '3602', true, '3201',
   '翠綠農場', '0912345678', NULL, NULL, NULL, NULL,
   '翠綠農場', '0912345678', NULL, NULL, NULL, NULL);

-- 店家 1004（澎湖鮮味水產）- 黑貓宅急便
INSERT INTO seller_shipment_settings (
  id, store_id, shipping_type, mer_id,
  onboarding_status, enabled, goods_type,
  sender_name, sender_phone, sender_city, sender_district, sender_address_detail, sender_postal_code,
  return_name, return_phone, return_city, return_district, return_address_detail, return_postal_code
) VALUES
  (10003, 1004, '3652', 'HAWK4',
   '3602', true, NULL,
   '澎湖水產', '0923456789', '澎湖縣', '馬公市', '中正路50號', '880',
   '澎湖水產', '0923456789', '澎湖縣', '馬公市', '中正路50號', '880');

COMMIT;
