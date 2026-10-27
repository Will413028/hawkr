-- Issue 3：ORDER_* 系列模板重整
--
-- 1. data_type 精細化（5 row）：通用 ORDER → 子類型
--    讓前端 NotificationRouter 能精準切換 UI / 跳轉頁面
--
-- 2. push_mode 改 silent（6 row）：動作執行者一律 silent，避免噪音
--    例：賣家自己出貨的不該再收「我已出貨」banner
--
-- 3. 新增 refund.completed → seller silent template（1 row）
--    買家看到退款是好消息（visible），賣家已知道（同意退款那步），純 refresh

-- Step 1: data_type 精細化 ============================================
UPDATE "notification_templates" SET "data_type" = 'ORDER_DELIVERED'
  WHERE "routing_key" = 'shipment.delivered'
    AND "target_user_type" IN ('buyer', 'seller');

UPDATE "notification_templates" SET "data_type" = 'ORDER_REFUNDED'
  WHERE "event_type" = 'refund.completed'
    AND "target_user_type" = 'buyer';

UPDATE "notification_templates" SET "data_type" = 'PAYOUT_COMPLETED'
  WHERE "event_type" = 'payout.completed'
    AND "target_user_type" = 'seller';

UPDATE "notification_templates" SET "data_type" = 'RETURN_SHIPPED'
  WHERE "event_type" = 'return.shipped'
    AND "target_user_type" = 'seller';

-- Step 2: push_mode = 'silent' ========================================
-- 動作執行者收 silent；對方/系統好消息維持 visible
UPDATE "notification_templates" SET "push_mode" = 'silent'
  WHERE "event_type" IN (
    'order.shipped.merchant',           -- 賣家自己出貨
    'shipment.delivered.seller',        -- 賣家視角包裹送達（系統事件，但賣家無動作要做）
    'order.completed.buyer',            -- 訂單自然完成（買家視角無動作）
    'order.completed.merchant',         -- 訂單自然完成（賣家視角無動作）
    'order.cancelled.merchant',         -- 賣家自己取消
    'order.refund.requested.buyer'      -- 買家自己申請售後
  );

-- Step 3: 新增 refund.completed → seller silent template ==============
INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled", "push_mode"
) VALUES (
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22),
  'hawkr.payment', 'refund.completed', 'notification.refund.completed.seller',
  'refund.completed.seller', 'seller', 'sellerAccountId',
  'refund_update',
  '退款完成',
  '訂單 ##{orderNo} 退款已完成。',
  NULL,
  'ORDER_REFUNDED',
  '{"refType": {"const": "ORDER"}, "refId": {"field": "orderNo"}, "order_id": {"field": "orderNo"}}'::jsonb,
  'hawkr://orders/{order_id}',
  'refund_seller_#{orderNo}',
  NULL, true, 'silent'
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
