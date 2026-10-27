UPDATE notification_templates
SET reference_id_template = '#{order_no}'
WHERE routing_key = 'shipment.delivered'
  AND target_user_type = 'buyer'
  AND reference_id_template = '#{orderSn}';
UPDATE notification_templates
SET target_user_field    = 'buyer_account_id',
    reference_id_template = '#{order_no}',
    image_url_template    = '#{seller_avatar}',
    data_fields           = '{"refId": {"field": "order_no"}, "orderId": {"field": "order_no"}, "refType": {"const": "ORDER"}}'::jsonb
WHERE routing_key = 'shipment.status.changed'
  AND target_user_type = 'buyer'
  AND target_user_field = 'buyerAccountId';
