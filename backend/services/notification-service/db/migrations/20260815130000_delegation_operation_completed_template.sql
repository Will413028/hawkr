-- 賣家「平台已代為操作」通知 — DB-driven template（56 號設計書 §6.3 / §4.5）。
--
-- Producer : account-service admin_operation_outbox dispatcher
--            （exchange=hawkr.delegation, routing_key=delegation.operation.completed）
-- Consumer : notification-service 模板引擎（push_mode='visible'：寫 inbox + FCM banner）
--
-- target_user_field 是 target_seller_account_id 而不是 target_store_id：賣家收件匣的讀取端
-- （gateway/internal/handler/notification_inbox.go）對 user_type != 'buyer' 直接拿 X-Account-Id
-- 當 user_id，不解析 store surrogate。payload 兩欄都在，填錯不會報錯——首任店家
-- （store_id == account_id）測起來全綠，re-key 過的店家永遠收不到且無告警。
--
-- data_fields / schema_url 皆為 NULL：本期賣家側零改動，沒有代行紀錄的落地頁，給了 deep link
-- 只會把賣家導向不存在的頁面。可用變數恰為三個（action_label / resource_name /
-- actor_display_name），其餘 payload 欄位是內部識別碼與稽核用值，不進賣家端文案。
--
-- 唯一鍵是 (event_type, target_user_type) 且表無 channel 欄位：本事件只會有這一列，
-- 通道完全由 push_mode 決定。平台沒有 Email 通道（§1.3 Non-Goal）。
INSERT INTO "notification_templates" (
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "enabled", "push_mode"
) VALUES (
  'hawkr.delegation', 'delegation.operation.completed',
  'delegation.operation.completed.seller', 'seller', 'target_seller_account_id',
  'system',
  '平台已代您操作商店',
  '平台管理員 #{actor_display_name} 已代您執行「#{action_label}」，對象：#{resource_name|您的商店}。',
  '平台管理員已代您執行一項商店操作，如有疑問請聯繫客服。',
  'DELEGATION_OPERATION_COMPLETED', NULL, NULL, '#{audit_log_id}',
  true, 'visible'
);
