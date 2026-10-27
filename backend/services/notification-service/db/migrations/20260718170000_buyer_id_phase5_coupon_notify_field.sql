UPDATE "notification_templates"
SET target_user_field = 'notify_buyer_account_id'
WHERE event_type = 'coupon.expiring';
