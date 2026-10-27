UPDATE "notification_templates"
SET target_user_field = 'notify_seller_account_id'
WHERE event_type = 'order.completed.merchant';
