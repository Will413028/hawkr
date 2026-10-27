-- #1226 修正：return_in_transit template 沿用了舊（2026-06-17 snake_case 改造前）的 camelCase
-- placeholder / target_user_field，導致 notification-service extractUserID 找不到 sellerAccountId、
-- 占位符 #{caseNo} 無法替換。對齊既有已修正 template（如 return_at_store）的 snake_case 格式。
-- 症狀：賣家「買家已寄出退貨」FCM/inbox 通知靜默不發（realtime SSE 不受影響，走獨立 channel）。

UPDATE "notification_templates"
SET target_user_field = 'seller_account_id',
    body_template = '買家已寄出退貨商品，目前配送中，請留意收貨（案件 #{case_no}）',
    data_fields = '{"refType": {"const": "AFTERSALE"}, "refId": {"field": "case_no"}, "caseNo": {"field": "case_no"}, "orderNo": {"field": "order_no"}, "actor": {"field": "actor"}, "shippingMethod": {"field": "shipping_method"}}',
    reference_id_template = 'case_return_in_transit_#{case_no}'
WHERE routing_key = 'aftersales.case.return_in_transit';
