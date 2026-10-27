-- #1225 確認收貨自動取消售後：新增「確認收貨取消」賣家側 FCM/inbox template
--   routing_key: aftersales.case.cancelled_by_confirm_receipt（A 桶 PENDING(0)/REOPENED(65) → CANCELLED_BY_CONFIRM_RECEIPT(71)）
--   買家於售後待處理階段主動確認收貨，系統自動取消售後案件；賣家需知悉案件已自動取消。
--   買家側不發（買家自身觸發，且訂單隨即完成，silent）。
-- snake_case placeholder / target_user_field（對齊 2026-06-17 後既有 template 格式）
-- 詳見 09 詳細設計書 §2.1.1 / §4.4 與 36 MQ Catalog §3.4

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
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 112,
  'aftersales.events', 'aftersales.case.cancelled_by_confirm_receipt', 'notification.aftersales.case.cancelled_by_confirm_receipt.seller',
  'aftersales.case.cancelled_by_confirm_receipt.seller', 'seller', 'seller_account_id',
  'order_update',
  '售後案件已自動取消',
  '買家確認收貨，售後申請已自動取消（案件 #{case_no}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "case_no"}, "caseNo": {"field": "case_no"}, "orderNo": {"field": "order_no"}, "actor": {"field": "actor"}}',
  'hawkr://aftersales/{case_no}',
  'case_cancelled_by_confirm_receipt_#{case_no}',
  NULL, true, 'visible'
)

ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
