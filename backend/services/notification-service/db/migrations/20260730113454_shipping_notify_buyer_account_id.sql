UPDATE "notification_templates"
SET target_user_field = 'notify_buyer_account_id'
WHERE event_type IN ('shipment.status.changed', 'shipment.delivered')
  AND target_user_type = 'buyer';
