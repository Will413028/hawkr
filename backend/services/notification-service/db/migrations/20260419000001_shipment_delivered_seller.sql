-- 新增通知模板：shipment.delivered → seller（#434）
INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled"
) VALUES (
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22),
  'hawkr.shipping', 'shipment.delivered', 'notification.shipment.delivered.seller',
  'shipment.delivered.seller', 'seller', 'sellerAccountId',
  'order_update',
  '訂單已送達',
  '訂單 #{orderNo} 已送達，等待買家確認收貨。',
  '您有訂單已送達，請留意買家確認收貨。',
  'ORDER',
  '{"order_id": "orderNo"}',
  'hawkr://orders/{order_id}',
  '#{orderNo}',
  NULL, true
);
