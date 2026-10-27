-- 2026-06-08：賣家「被追蹤」推播 body 顯示收藏買家暱稱（拿不到 → 退回「有人」）。
-- 舊 body 為 '#{followerName} 開始追蹤了！'（無 default，名字空時開頭多一格空白）。
-- 改用 per-field default 語法，由 notification-service RenderTemplate（#914）兜底。
-- buyer-service 已在 follow.added 事件 best-effort enrich followerName。
UPDATE "notification_templates"
SET "body_template" = '#{followerName|有人} 開始追蹤了！'
WHERE "routing_key" = 'follow.added'
  AND "target_user_type" = 'seller';
