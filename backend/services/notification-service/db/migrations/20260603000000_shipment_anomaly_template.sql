-- §5.21 配送異常賣家通知 — DB-driven template
--
-- 設計書：06 §5.21（shipping-service）、12 §3.1/§3.4/§4.2 #11（notification-service）、36 §3.3（MQ catalog）
--
-- Producer  : shipping-service（5 個偵測點共用 ShipmentAnomalyEventPublisher）
--             + aftersales-service（RETURN_STUCK 透過 RabbitTemplate 寫 hawkr.shipping）
-- Consumer  : notification-service 模板引擎（push_mode='visible'：寫 inbox + FCM banner）
-- Category  : shipment_anomaly（賣家強制 on 不可關 — 由 domain/usecase 層拒絕關閉）
-- Idempotency: reference_id_template = '#{orderNo}|#{anomalyType}'
--             同 (orderNo, anomaly_type) 唯一 inbox row + 唯一 FCM send；scheduler 每輪重複 publish 自然去重
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
  'hawkr.shipping', 'shipment.anomaly.detected', 'notification.shipment.anomaly',
  'shipment.anomaly.detected.seller', 'seller', 'sellerAccountId',
  'shipment_anomaly',
  '配送異常通知',
  '訂單 #{orderNo} #{anomalyTypeLabel}，請儘速處理。',
  '您有訂單配送異常，請至異常 dashboard 查看。',
  'SHIPMENT_ANOMALY',
  '{"refType": {"const": "SHIPMENT_ANOMALY"}, "refId": {"field": "orderNo"}, "order_no": {"field": "orderNo"}, "anomaly_type": {"field": "anomalyType"}, "severity": {"field": "severity"}}',
  -- 跳到該訂單詳情頁（既有 router 都有對應）：seller mobile `OrderRoutes.detail` (`/orders/:orderNo`)、
  -- web `/seller-center/orders/:orderNo`。訂單詳情頁已有紅角異常標示，賣家可從該頁進入異常 dashboard。
  -- 不採 `hawkr://shipment-anomalies/:orderNo` 因為現階段無單筆 anomaly detail 頁。
  'hawkr://orders/{order_no}',
  '#{orderNo}|#{anomalyType}',
  NULL, true, 'visible'
);
