-- 2026-05-22 逆物流退貨政策修訂：新增 6 個賣家側 FCM template
--   1. return_at_store              超商到店待取（72h 提醒）
--   2. return_at_store_24h          超商待取剩 24h 預警
--   3. return_delivered             已修改既有的 return_delivered template body（保留歷史相容）
--   4. return_delivered_24h         確認收貨剩 24h 預警
--   5. auto_completed_store_pickup_timeout    超商待取逾時自動完成
--   6. auto_completed_confirm_receipt_timeout 確認收貨逾時自動完成

-- 詳見 09 詳細設計書 §5.3.14 與 36 MQ Catalog §3.4

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
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 100,
  'aftersales.events', 'aftersales.case.return_at_store', 'notification.aftersales.case.return_at_store.seller',
  'aftersales.case.return_at_store.seller', 'seller', 'sellerAccountId',
  'order_update',
  '退貨商品待領取',
  '退貨商品已送達至退貨門市，請於 72 小時內完成取貨，逾時系統將自動確認收貨並退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "shippingMethod": {"field": "shippingMethod"}, "deadlineAt": {"field": "deadlineAt"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_at_store_#{caseNo}',
  NULL, true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 101,
  'aftersales.events', 'aftersales.case.return_at_store_24h', 'notification.aftersales.case.return_at_store_24h.seller',
  'aftersales.case.return_at_store_24h.seller', 'seller', 'sellerAccountId',
  'order_update',
  '退貨商品待領取',
  '退貨商品已送達至退貨門市，請於 24 小時內完成取貨，逾時系統將自動確認收貨並退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "shippingMethod": {"field": "shippingMethod"}, "deadlineAt": {"field": "deadlineAt"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_at_store_24h_#{caseNo}',
  NULL, true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 102,
  'aftersales.events', 'aftersales.case.return_delivered', 'notification.aftersales.case.return_delivered.seller',
  'aftersales.case.return_delivered.seller', 'seller', 'sellerAccountId',
  'order_update',
  '退貨商品已送達',
  '請於 48 小時內至訂單管理確認收貨，逾時系統將自動確認收貨並退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "shippingMethod": {"field": "shippingMethod"}, "deadlineAt": {"field": "deadlineAt"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_delivered_#{caseNo}',
  NULL, true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 103,
  'aftersales.events', 'aftersales.case.return_delivered_24h', 'notification.aftersales.case.return_delivered_24h.seller',
  'aftersales.case.return_delivered_24h.seller', 'seller', 'sellerAccountId',
  'order_update',
  '退貨商品已送達',
  '請於 24 小時內至訂單管理確認收貨，逾時系統將自動確認收貨並退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "shippingMethod": {"field": "shippingMethod"}, "deadlineAt": {"field": "deadlineAt"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_delivered_24h_#{caseNo}',
  NULL, true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 104,
  'aftersales.events', 'aftersales.case.auto_completed_store_pickup_timeout', 'notification.aftersales.case.auto_completed_store_pickup_timeout.seller',
  'aftersales.case.auto_completed_store_pickup_timeout.seller', 'seller', 'sellerAccountId',
  'order_update',
  '售後單已自動完成',
  '未於平台指定時間內完成相關操作，系統已自動確認收貨並退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "autoCompleteReason": {"field": "autoCompleteReason"}, "autoCompleteReasonLabel": {"field": "autoCompleteReasonLabel"}}',
  'hawkr://aftersales/{caseNo}',
  'case_auto_completed_store_#{caseNo}',
  NULL, true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 105,
  'aftersales.events', 'aftersales.case.auto_completed_confirm_receipt_timeout', 'notification.aftersales.case.auto_completed_confirm_receipt_timeout.seller',
  'aftersales.case.auto_completed_confirm_receipt_timeout.seller', 'seller', 'sellerAccountId',
  'order_update',
  '售後單已自動完成',
  '未於平台指定時間內完成相關操作，系統已自動確認收貨並退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}, "autoCompleteReason": {"field": "autoCompleteReason"}, "autoCompleteReasonLabel": {"field": "autoCompleteReasonLabel"}}',
  'hawkr://aftersales/{caseNo}',
  'case_auto_completed_confirm_#{caseNo}',
  NULL, true, 'visible'
)

ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
