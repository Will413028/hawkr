-- #1226 售後 realtime 覆蓋補齊：新增「退貨運送中」賣家側 FCM/inbox template
--   routing_key: aftersales.case.return_in_transit（RETURN_PENDING 20 → RETURN_SHIPPING 25）
--   原本此轉換不發任何 domain event，賣家收不到「買家已寄出退貨」通知，亦無賣家工作區 realtime
-- 詳見 09 詳細設計書 §2.1 / §5.3.4 與 36 MQ Catalog §3.4

INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled", "push_mode"
) VALUES
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 110,
  'aftersales.events', 'aftersales.case.return_in_transit', 'notification.aftersales.case.return_in_transit.seller',
  'aftersales.case.return_in_transit.seller', 'seller', 'sellerAccountId',
  'order_update',
  '買家已寄出退貨',
  '買家已寄出退貨商品，目前配送中，請留意收貨（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "shippingMethod": {"field": "shippingMethod"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_in_transit_#{caseNo}',
  NULL, true, 'visible'
)

ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
