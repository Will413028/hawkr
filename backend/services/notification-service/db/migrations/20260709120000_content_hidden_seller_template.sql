-- 賣家「貼文下架」通知 — DB-driven template（POST_HIDDEN_BY_PLATFORM）
--
-- 設計書：docs/superpowers/specs/2026-07-09-seller-content-takedown-notification-design.md
-- Producer  : social-service（PostHider.HidePost → PublishPostHiddenByPlatform，
--             exchange=hawkr.content, routing_key=post.hidden_by_platform，JSON）
-- Consumer  : notification-service 模板引擎（push_mode='visible'：寫 inbox + FCM banner）
-- 新 queue（notification.post.hidden_by_platform）→ 部署需重啟才會 declare+bind。
-- 事件 payload 為 snake_case：seller_id / post_id / reason。
-- reason 空時走 per-field 預設 #{reason|未提供}；key 全缺時走整句 body_fallback。
-- schema_url 深連結至賣家自己的貼文（含已下架）；若 seller-app 無此 route 則改 NULL。
-- id 為顯式 TSID（表無 default，NOT NULL）；+201 為避免同毫秒碰撞的偏移。
INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled", "push_mode"
) VALUES (
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 201,
  'hawkr.content', 'post.hidden_by_platform', 'notification.post.hidden_by_platform',
  'post.hidden_by_platform.seller', 'seller', 'seller_id',
  'system',
  '您的貼文已被下架',
  '您的貼文因違反平台規範已被下架，原因：#{reason|未提供}',
  '您的貼文因違反平台規範已被下架，如有疑問請聯繫客服。',
  'POST_HIDDEN_BY_PLATFORM',
  '{"refType": {"const": "POST"}, "refId": {"field": "post_id"}, "post_id": {"field": "post_id"}, "reason": {"field": "reason"}}',
  'hawkr://posts/{post_id}',
  'content_hidden_#{post_id}',
  NULL, true, 'visible'
);
