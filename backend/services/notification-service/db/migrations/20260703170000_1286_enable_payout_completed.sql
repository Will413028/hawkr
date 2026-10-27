-- issue #1286: 啟用 payout.completed 賣家撥款通知（order-service GrantPaymentWorker emit）。
-- 改綁 hawkr.order exchange（order-service 發）、enable、read-key 遷 snake、加撥款金額。
-- push_mode=visible → notification-service 寫 inbox + FCM 推播(手機) + emit notification.inbox.new(→ web SSE)，雙通道一致。
-- data_fields 左鍵(FCM 契約)保 camelCase；右值/target/#{placeholder} 用 L1 snake（36 §7.4.1）。

UPDATE notification_templates SET
  exchange = 'hawkr.order',
  target_user_field = 'seller_account_id',
  body_template = '訂單 ##{order_no} 款項 NT$#{payout_amount} 已撥入您的錢包，點此確認明細。',
  reference_id_template = 'payout_#{order_no}',
  data_fields = '{"refId": {"field": "order_no"}, "orderId": {"field": "order_no"}, "refType": {"const": "ORDER"}}',
  push_mode = 'visible',
  enabled = true,
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'payout.completed' AND target_user_type = 'seller';
