-- issue #1508: re-enable 買家「抵達門市/投遞通知」(shipment.delivered, target_user_type=buyer)。
-- 13 號設計書:254 與 12 號設計書:1025-1048 均設計為正常買家通知，文案/routing 從未變更過。
-- 2026-06-25 dedup 事故（7dd16e24e）止血時手動關閉（updated_by=NULL，無 audit 記錄可考），
-- 事後忘記 re-enable，造成 buildDynamicSubscriptions（consumer.go）不再為此 template 建訂閱，
-- notification.shipment.delivered queue 無 consumer，訊息滯留（10 筆）。本次修復方向 = 單純
-- re-enable，template 內容本身不變。
UPDATE notification_templates SET
  enabled = true,
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'shipment.delivered' AND target_user_type = 'buyer';
