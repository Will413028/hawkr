-- issue #1176 PR-4: coupon.expiring template 遷 snake_case read-key，對齊新 publisher
-- （coupon-service CouponExpiringWarningJob 發 snake payload）。
-- data_fields 左鍵（FCM 契約）保持既有 camelCase；只改右側 field 值與 #{placeholder}。

UPDATE notification_templates SET
  target_user_field = 'buyer_account_id',
  body_template = '⏰ 快過期了！您的「#{coupon_name}」將於今晚失效，把握最後機會！',
  reference_id_template = 'expiring_#{coupon_instance_id}_#{date_key}',
  image_url_template = '#{coupon_image_url}',
  data_fields = '{"refId": {"field": "coupon_instance_id"}, "refType": {"const": "COUPON"}, "couponId": {"field": "coupon_instance_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'coupon.expiring' AND target_user_type = 'buyer';
