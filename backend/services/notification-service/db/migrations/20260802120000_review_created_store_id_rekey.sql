UPDATE "notification_templates"
SET reference_id_template = 'review_#{order_no}_#{store_id}',
    updated_at            = CURRENT_TIMESTAMP
WHERE event_type = 'review.created';
