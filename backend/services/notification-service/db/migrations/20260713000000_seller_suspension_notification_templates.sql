-- 賣家停權連動通知 — DB-driven template（44 §5.10a saga / 03 §5.1a.2 / 07 §5.20 / 36 §3.6f）
-- 設計書：docs/詳細設計書/12-通知微服務（Notification Service）詳細設計書.md §7.10
--
-- 通知 1：賣家停權（seller.suspended → 賣家本人）
-- 通知 2：賣家解除停權（seller.unsuspended → 賣家本人）
--   Producer  : seller-service（SuspendSellerForAdmin / LiftSellerSuspensionForAdmin，走 outbox 必達）
--   Consumer  : notification-service 模板引擎（新 queue notification.seller.suspension，
--               同 queue 綁定兩個 routing key，各自一筆 template）
--   Payload   : SellerSuspendedEvent/SellerUnsuspendedEvent（pkg/mq/events.go）——
--               suspensionId / accountId 皆為 JSON string（TSID 精度，CLAUDE.md §4），
--               extractUserID 支援字串型別欄位，target_user_field 可直接用 accountId。
--
-- 通知 3：訂單因賣家停權取消（order.cancelled + payload_filter → 買家）
--   Producer  : order-service OrderService.publishStockReleaseEvent（複用既有 order.cancelled 發布點；
--               本 PR 於 OrderCancelledPayload 新增 cancelReason 欄位，snake_case wire = cancel_reason）
--   Consumer  : notification-service 模板引擎，掛在既有 queue notification.order.events 上，
--               以 payload_filter 篩 cancel_reason=9104（OrderCancelReason.SELLER_SUSPENDED 的
--               CodeMeta *value*，非 code 字面 "SELLER_SUSPENDED"——DB/wire 實際存放的是 value）。
--
-- ⚠ 已知限制（機制與設計書 §7.10 描述有落差，記錄不硬湊）：
-- 設計書原文「兩者靠 reference_id＝orderNo 冪等」暗示本模板與既有 order.cancelled.buyer（id=7）
-- 會因同 reference_id 互斥只擇一送達；但 notification_inbox 的冪等鍵實際是
-- (user_id, user_type, event_type, reference_id) 四元組（見 internal/usecase/notification_inbox.go），
-- 兩筆 template 的 event_type 不同（order.cancelled.buyer vs order.cancelled.seller_suspended），
-- 不會互相去重。payload_filter 引擎（matchesFilter）只支援正向相等匹配，無排除語法，
-- 現有 order.cancelled.buyer 模板無法設定「cancel_reason ≠ 9104 才觸發」。
-- 實際行為：賣家停權取消訂單時，買家會同時收到「訂單已取消」（通用）與「訂單因賣家停權已取消」
-- （本模板）兩則 inbox 通知，而非設計書預期的單一則。修正需 Go 端 matchesFilter 支援排除語法或
-- 改變 order.cancelled.buyer 的觸發條件，屬 Go code 變更，本 PR 範圍（notification 不改 Go code）
-- 不處理，留待後續 PR 決策（接受重複 or 擴充引擎）。
INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key", "queue_name",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "image_url_template", "payload_filter",
  "enabled", "push_mode"
) VALUES
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 401,
  'hawkr.seller', 'seller.suspended', 'notification.seller.suspension',
  'seller.suspended.notice', 'seller', 'accountId',
  'seller_account',
  '您的賣家帳號已被停權',
  '原因代碼：#{reasonCode}。停權期間商品已暫時下架、未付款訂單已取消，如有疑問請至申訴頁提出申訴。',
  '您的賣家帳號已被停權，如有疑問請至申訴頁提出申訴。',
  'SELLER_SUSPENDED',
  '{"refType": {"const": "SELLER_SUSPENSION"}, "refId": {"field": "suspensionId"}, "reasonCode": {"field": "reasonCode"}}',
  NULL,
  '#{suspensionId}',
  NULL, NULL, NULL,
  true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 402,
  'hawkr.seller', 'seller.unsuspended', 'notification.seller.suspension',
  'seller.unsuspended.notice', 'seller', 'accountId',
  'seller_account',
  '您的賣家帳號停權已解除',
  '停權已解除，您可恢復正常營運。',
  '您的賣家帳號停權已解除。',
  'SELLER_UNSUSPENDED',
  '{"refType": {"const": "SELLER_SUSPENSION"}, "refId": {"field": "suspensionId"}}',
  NULL,
  '#{suspensionId}',
  NULL, NULL, NULL,
  true, 'visible'
),
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 403,
  'hawkr.order', 'order.cancelled', 'notification.order.events',
  'order.cancelled.seller_suspended', 'buyer', 'buyer_account_id',
  'order_update',
  '訂單已取消（賣家停權）',
  '訂單 ##{order_no} 因賣家帳號停權已為您取消，若已付款將自動原路退回，無需另行處理。',
  '您的訂單因賣家帳號停權已取消，若已付款將自動原路退回。',
  'ORDER_CANCELLED',
  '{"refId": {"field": "order_no"}, "orderId": {"field": "order_no"}, "refType": {"const": "ORDER"}}',
  'hawkr://orders/{order_id}',
  '#{order_no}',
  NULL, '#{seller_avatar}', '{"cancel_reason": "9104"}',
  true, 'visible'
);
