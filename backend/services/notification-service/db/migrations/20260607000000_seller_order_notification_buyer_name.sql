-- 2026-06-07：賣家「新訂單」推播 body 顯示下單買家暱稱（拿不到 → 退回「買家」）。
-- 推播橫幅（App 背景時 OS 直接顯示 server body）原本無買家名字；改用 per-field default
-- 語法 #{buyerNickname|買家}，payload 已由 shipping-service best-effort enrich buyerNickname。
-- title 維持不變（in-app 個人化交給前端 _personalize）。issue 延續 #686。
UPDATE "notification_templates"
SET "body_template" = '🧧 #{buyerNickname|買家} 已下單付款，請儘速準備商品並在 2 天內出貨。'
WHERE "queue_name" = 'notification.order.placed'
  AND "event_type" = 'shipment.created'
  AND "target_user_type" = 'seller';
