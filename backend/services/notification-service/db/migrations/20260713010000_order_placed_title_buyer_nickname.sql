-- 2026-07-13：賣家「新訂單」OS 背景推播橫幅的 title 固定顯示通用「買家已下單」，
-- 沒有把買家暱稱嵌入 title（僅 body 有 #{buyer_nickname|買家} placeholder，見 20260624141029）。
-- 對齊 body_template 用法，title 一併帶入買家暱稱，查無暱稱時 fallback 回「買家」。
UPDATE notification_templates
SET title_template = '#{buyer_nickname|買家}已下單'
WHERE queue_name = 'notification.order.placed'
  AND event_type = 'shipment.created'
  AND target_user_type = 'seller';
