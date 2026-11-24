-- B3 step 4：Java shipping-service 刪除，出貨通知隨出貨紀錄在 B8 重建
-- （ADR 2026-11-15，2026-11-24 修訂）。
--
-- 賣家「新訂單」推播原綁 hawkr.shipping:shipment.created（20260531000000），
-- shipment.created 不再有人發，改回綁 hawkr.order:order.paid。
-- data_fields 的來源欄位 order_no / buyer_nickname / buyer_avatar_key 與
-- target_user_field store_id 都在 Java order-service 的 OrderPaidPayload 內。
-- event_type 改為 order.paid.seller：order.paid 已是買家 template 的 event_type
-- （contract golden 以 event_type 為 key），賣家沿用「<事件>.seller」的命名。
-- queue 由 consumer 依 exchange 對應（hawkr.order → notification.order）。
UPDATE "notification_templates"
SET
  "exchange"    = 'hawkr.order',
  "routing_key" = 'order.paid',
  "event_type"  = 'order.paid.seller',
  "updated_at"  = CURRENT_TIMESTAMP
WHERE "exchange"         = 'hawkr.shipping'
  AND "target_user_type" = 'seller'
  AND "event_type"       = 'shipment.created';

-- 其餘出貨模板（shipment.delivered 買賣家、shipment.anomaly.detected、
-- shipment.status.changed 各變體）連同 hawkr.shipping 的整條事件一起移除。
DELETE FROM "notification_templates" WHERE "exchange" = 'hawkr.shipping';

-- notification_preferences.category 是自由字串、沒有外鍵；移除已不存在的分類偏好列。
DELETE FROM "notification_preferences" WHERE "category" IN ('shipment_anomaly', 'shipment');
