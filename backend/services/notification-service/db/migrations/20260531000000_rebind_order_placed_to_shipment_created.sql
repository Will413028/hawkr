-- 2026-05-31：賣家「新訂單」推播觸發點由 hawkr.order:order.paid 改綁
-- hawkr.shipping:shipment.created。
--
-- 背景：原綁 order.paid 會在「付款瞬間」就推播賣家，但若 shipping
-- createShipmentFromOrder 失敗（PayUni 拒絕、seller 未開通等），賣家收到推播
-- 卻打開 App 沒物流單可操作；30 分鐘後 order.shipment.create_requested
-- Backstop 救活時又因 notification 沒有對應 binding，無法補發推播。
--
-- 改綁 shipment.created 後：
--   - 正常付款：建單僅數秒，賣家體感無延遲。
--   - Backstop 救活：賣家在「救活當下」被推播（時機正確）。
--   - 永久失敗：shipment.created 永不發出，賣家不會被推播假象訂單。
--
-- order.paid 仍保留給「買家」推播（notification.order.paid queue，不動）。
--
-- issue #686 個人化字樣保留：shipping-service 在 publishShipmentCreatedEvent 內
-- 透過 BuyerGrpcClient.batchGetBuyerMeta best-effort enrich 取得 buyerNickname /
-- buyerAvatar 後寫入 ShipmentCreatedPayload；enrich 失敗時兩欄為 null，FieldSpec.Resolve
-- 略過該欄位，前端 _personalize helper fallback 至通用「買家」。本 migration
-- data_fields 維持原 5 個 key（含 buyerNickname / buyerAvatar），與原 order.paid
-- 賣家 template 行為一致。
--
-- RabbitMQ 舊綁定（hawkr.order:order.paid → notification.order.placed）
-- 不會自動清除（consumer 啟動只 QueueBind 不 QueueUnbind）。本 migration apply 後
-- 須於 RabbitMQ 手動 unbind，否則舊 order.paid 訊息會繼續灌進該 queue
-- 並在 template_handler 找不到匹配 template 後 ack-drop（warn log）。

UPDATE "notification_templates"
SET
  "exchange"    = 'hawkr.shipping',
  "routing_key" = 'shipment.created',
  "event_type"  = 'shipment.created',
  "data_fields" = '{
    "refType":       {"const": "ORDER"},
    "refId":         {"field": "orderNo"},
    "order_id":      {"field": "orderNo"},
    "buyerNickname": {"field": "buyerNickname"},
    "buyerAvatar":   {"field": "buyerAvatar"}
  }'::jsonb
WHERE "queue_name"       = 'notification.order.placed'
  AND "target_user_type" = 'seller'
  AND "event_type"       = 'order.placed';
