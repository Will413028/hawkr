UPDATE "notification_templates"
SET target_user_field = 'notify_buyer_account_id'
WHERE event_type IN ('order.paid', 'order.cancelled.buyer', 'order.cancelled.seller_suspended', 'order.completed.buyer');
