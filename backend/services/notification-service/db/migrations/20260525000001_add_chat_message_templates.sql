-- B-178：聊天離線推播。im-service 發 hawkr.im / chat.message.sent 事件，
-- 由本模板驅動「可見通知 + data」FCM 推播（push_mode=visible），復用 token 查詢 /
-- 壞 token 清理 / inbox / 冪等 pipeline。receiver 可能是 buyer 或 seller，故用
-- payload_filter 的 receiverType 拆成兩筆，各自鎖定 target_user_type。
--
-- data 欄位走 camelCase，對齊前端契約（senderName / conversationId / messageId / sentAt）。
-- 圖片訊息的 contentPreview 由 im-service 端改為「傳送了一張圖片」，body 不會出現 CDN URL。
INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "image_url_template", "payload_filter",
  "enabled", "push_mode"
) VALUES
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22),
  'hawkr.im', 'chat.message.sent', 'notification.chat.message.sent',
  'chat.message.sent', 'buyer', 'receiverId',
  'chat',
  '#{senderName}',
  '#{contentPreview}',
  '您有一則新訊息',
  'chat_message',
  '{"refType": {"const": "CHAT"}, "refId": {"field": "conversationId"}, "senderId": {"field": "senderId"}, "senderName": {"field": "senderName"}, "senderAvatar": {"field": "senderAvatar"}, "conversationId": {"field": "conversationId"}, "messageId": {"field": "messageId"}, "sentAt": {"field": "sentAt"}}',
  NULL,
  '#{messageId}',
  NULL, NULL, '{"receiverType": "buyer"}',
  true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 1,
  'hawkr.im', 'chat.message.sent', 'notification.chat.message.sent',
  'chat.message.sent', 'seller', 'receiverId',
  'chat',
  '#{senderName}',
  '#{contentPreview}',
  '您有一則新訊息',
  'chat_message',
  '{"refType": {"const": "CHAT"}, "refId": {"field": "conversationId"}, "senderId": {"field": "senderId"}, "senderName": {"field": "senderName"}, "senderAvatar": {"field": "senderAvatar"}, "conversationId": {"field": "conversationId"}, "messageId": {"field": "messageId"}, "sentAt": {"field": "sentAt"}}',
  NULL,
  '#{messageId}',
  NULL, NULL, '{"receiverType": "seller"}',
  true, 'visible'
);
