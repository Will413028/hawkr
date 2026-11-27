
BEGIN;

DELETE FROM seller_shipment_settings WHERE store_id IN (1009, 1004);

INSERT INTO seller_shipment_settings (
  id, store_id, shipping_type, mer_id,
  onboarding_status, enabled, goods_type,
  sender_name, sender_phone, sender_city, sender_district, sender_address_detail, sender_postal_code,
  return_name, return_phone, return_city, return_district, return_address_detail, return_postal_code
) VALUES
  (10001, 1009, '3652', 'HAWK9',
   '3602', true, NULL,
   '翠綠農場', '0912345678', '台南市', '新化區', '中山路100號', '712',
   '翠綠農場', '0912345678', '台南市', '新化區', '中山路100號', '712'),
  (10002, 1009, '3651', 'HAWK9',
   '3602', true, '3201',
   '翠綠農場', '0912345678', NULL, NULL, NULL, NULL,
   '翠綠農場', '0912345678', NULL, NULL, NULL, NULL);

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
