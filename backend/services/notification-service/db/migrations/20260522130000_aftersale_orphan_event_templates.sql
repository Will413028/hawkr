-- 2026-05-22 補齊 3 個既有 aftersales 事件的 buyer/seller notification template
--   1. aftersales.case.return_failed (buyer visible + seller visible)
--   2. aftersales.refund.awaiting_buyer_page (buyer visible)
--   3. aftersales.case.auto_rejected (buyer visible)
--
-- 詳見 09 設計書 §4.4 與 36 MQ Catalog §3.4
-- cancel.completed 不在此檔範圍：下游 order-service 會發 order.cancelled 給 buyer，
-- 避免雙重推播

INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled", "push_mode"
) VALUES
-- ① return_failed × buyer：必補 — 物流失敗 buyer 完全不知，包裹遺失/廠退會卡死
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 200,
  'aftersales.events', 'aftersales.case.return_failed', 'notification.aftersales.case.return_failed.buyer',
  'aftersales.case.return_failed.buyer', 'buyer', 'buyerAccountId',
  'order_update',
  '退貨物流異常',
  '您的退貨包裹發生異常，請聯繫客服或前往申訴（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_failed_#{caseNo}',
  NULL, true, 'visible'
),
-- ② return_failed × seller：原本預期收到退貨，現在沒了，需主動處理或等 Admin
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 201,
  'aftersales.events', 'aftersales.case.return_failed', 'notification.aftersales.case.return_failed.seller',
  'aftersales.case.return_failed.seller', 'seller', 'sellerAccountId',
  'order_update',
  '退貨物流異常',
  '退貨包裹發生異常，請至訂單管理查看（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}}',
  'hawkr://aftersales/{caseNo}',
  'case_return_failed_#{caseNo}',
  NULL, true, 'visible'
),
-- ③ awaiting_buyer_page × buyer：必補 — ATM 退款必須買家去 PayUni 頁面填銀行帳號
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 202,
  'aftersales.events', 'aftersales.refund.awaiting_buyer_page', 'notification.aftersales.refund.awaiting_buyer_page.buyer',
  'aftersales.refund.awaiting_buyer_page.buyer', 'buyer', 'buyerAccountId',
  'order_update',
  '請填寫收款銀行帳號',
  '賣家已同意退款，請至訂單管理填寫收款銀行帳號以收取退款（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "refundTaskId": {"field": "refundTaskId"}}',
  'hawkr://aftersales/{caseNo}',
  'awaiting_buyer_page_#{caseNo}',
  NULL, true, 'visible'
),
-- ④ auto_rejected × buyer：必補 — 與賣家手動 rejected 對稱；文案區隔「賣家未回應由系統代拒」
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 203,
  'aftersales.events', 'aftersales.case.auto_rejected', 'notification.aftersales.case.auto_rejected.buyer',
  'aftersales.case.auto_rejected.buyer', 'buyer', 'buyerAccountId',
  'order_update',
  '售後申請已自動拒絕',
  '賣家未於期限內回應，售後申請已由系統代為拒絕，您可於申訴期限內提出申訴或重啟（案件 #{caseNo}）',
  '您有售後案件更新，請開啟 App 查看',
  'AFTER_SALE_STATUS',
  '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "caseNo"}, "caseNo": {"field": "caseNo"}, "orderNo": {"field": "orderNo"}, "actor": {"field": "actor"}}',
  'hawkr://aftersales/{caseNo}',
  'case_auto_rejected_#{caseNo}',
  NULL, true, 'visible'
)

ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
