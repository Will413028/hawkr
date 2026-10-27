-- 平台審核通過、剛轉送 PayUni 的中繼通知。此前 Approve() 在這個時間點錯發
-- "merchant.verification.approved"（見 backend-payuni docs/superpowers/specs/
-- 2026-07-01-seller-cert-payuni-review-gate-design.md），會讓賣家誤以為全部
-- 審核已經完成；改發獨立的 forwarded 事件，approved 模板保留給 PayUni 自己
-- 核准時才觸發（payuni.go 的 applyPayuniOutcome，語意本來就正確、不用動）。
-- id 為顯式 TSID（表無 default，NOT NULL）；merchantId / push_mode 對齊
-- merchant.verification.* 家族既有列。
INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled", "push_mode"
) VALUES (
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 113,
  'hawkr.merchant', 'merchant.verification.forwarded', 'notification.merchant.verification',
  'merchant.verification.forwarded', 'seller', 'merchantId',
  'system',
  '平台審核通過',
  '您的實名認證已通過平台審核，已送出至金流業者審核中，請耐心等候。',
  NULL,
  'VERIFICATION_UPDATE',
  NULL,
  NULL,
  'merchant_verification_forwarded_#{merchantId}',
  NULL, true, 'visible'
);
