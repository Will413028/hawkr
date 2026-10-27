-- 49 號 §6.3：product.restock.target payload 圖片欄位 wire key image_url→image_key（值改帶 objectKey）。
-- 對齊 producer（product-service ProductRestockPayload / buyer-service fan-out）與 pkg/mq wire tag。
-- image_key_template 改讀 #{image_key}；data_fields 圖片欄改讀 image_key（顯示 URL 由渲染端
-- mediaurl.BuildURL 現組於 data.imageUrl，BuildURL 對存量完整 URL 走 passthrough，遷移期不破圖）。
UPDATE notification_templates SET
  image_key_template = '#{image_key}',
  data_fields = '{"refId": {"field": "product_id"}, "refType": {"const": "PRODUCT"}, "deep_link": {"field": "deep_link"}, "image_key": {"field": "image_key"}, "product_id": {"field": "product_id"}, "subscription_id": {"field": "subscription_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'product.restock.target' AND target_user_type = 'buyer';
