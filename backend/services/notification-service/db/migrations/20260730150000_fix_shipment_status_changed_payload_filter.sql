-- shipment.status.changed.buyer（id=3，出貨通知）的 payload_filter 從建立
-- （20260703130000_template_ssot_baseline.sql）以來就是 {"status": "IN_TRANSIT"}，
-- 但 ShippingEventPayload 的 status 欄位一律用 ShippingStatus.value()（數字碼，
-- 如 "31"）寫進 wire payload，從未用 .code()（列舉名字串 "IN_TRANSIT"）——
-- matchesFilter 純字面字串比對，"31" != "IN_TRANSIT" 恆不相等，這個 template
-- 從建立起就沒有真正推播給任何買家過。改成比對真正會出現在 wire 上的數字碼，
-- 跟本表其他 payload_filter（皆比對 wire 字面值）的慣例一致。
UPDATE "notification_templates"
SET payload_filter = '{"status": "31"}'
WHERE event_type = 'shipment.status.changed'
  AND target_user_type = 'buyer';
