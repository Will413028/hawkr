-- 2026-06-24：補修 20260619000708 的漏網。賣家「買家已下單」背景推播橫幅
-- body 仍渲染通用「買家」而非下單買家暱稱。
--
-- 根因：20260619 的 body_template snake_case 遷移用字面
-- REPLACE(body_template, '#{buyerNickname}', '#{buyer_nickname}')，但本 row 的
-- placeholder 帶 per-field default（20260607 加的 '#{buyerNickname|買家}'），
-- '#{buyerNickname}' 字面不匹配 → 靜默跳過，camelCase 殘留。
-- 同次 migration 的 data_fields rename 操作的是 JSON 值 'buyerNickname'（無 |default 後綴），
-- 故 FCM data.buyerNickname 已修好——in-app _personalize 正常，僅背景推播橫幅
-- （OS 直接顯示 server body，RenderTemplate 直讀 snake_case payload，找不到
-- camelCase key）缺名字。
--
-- 將 placeholder 對齊 publisher 的 snake_case payload（buyer_nickname）。
UPDATE notification_templates
SET body_template = REPLACE(body_template, '#{buyerNickname|買家}', '#{buyer_nickname|買家}')
WHERE queue_name = 'notification.order.placed'
  AND event_type = 'shipment.created'
  AND target_user_type = 'seller';
