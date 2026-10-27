-- 新增通知模板：aftersales.case.auto_escalated → buyer + seller
-- 對應功能：售後處理期限自動升級（aftersales-service 排程任務）
-- 賣家未在期限內處理售後案件 → 平台接手仲裁，同時通知買賣雙方

INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled"
) VALUES
-- aftersales.case.auto_escalated → buyer
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22),
  'aftersales.events', 'aftersales.case.auto_escalated', 'notification.aftersales.auto_escalated.buyer',
  'aftersales.case.auto_escalated.buyer', 'buyer', 'buyerAccountId',
  'order_update',
  '平台已接手您的售後申請',
  '賣家未在期限內處理，平台已接手案件 #{caseNo}，將協助您完成處理',
  '您的售後案件已由平台接手',
  'AFTER_SALE_STATUS',
  '{"caseNo":"caseNo", "orderNo":"orderNo", "actor":"actor", "reason":"reason"}',
  'hawkr://aftersales/{caseNo}',
  'auto_escalated_#{caseNo}',
  NULL, true
),
-- aftersales.case.auto_escalated → seller
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 1,
  'aftersales.events', 'aftersales.case.auto_escalated', 'notification.aftersales.auto_escalated.seller',
  'aftersales.case.auto_escalated.seller', 'seller', 'sellerAccountId',
  'order_update',
  '您的售後申請已由平台接手',
  '您的售後案件 #{caseNo} 因 #{originalDeadlineHours} 小時內未處理，已由平台接手仲裁',
  '您的售後案件已由平台接手',
  'AFTER_SALE_STATUS',
  '{"caseNo":"caseNo", "orderNo":"orderNo", "actor":"actor", "reason":"reason"}',
  'hawkr://aftersales/{caseNo}',
  'auto_escalated_#{caseNo}',
  NULL, true
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
