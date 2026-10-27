UPDATE "notification_templates"
SET "data_fields" = jsonb_set("data_fields", '{sellerId,field}', '"store_id"'),
    "updated_at"  = CURRENT_TIMESTAMP
WHERE "event_type" = 'post.created'
  AND "target_user_type" = 'buyer'
  AND "data_fields" -> 'sellerId' ->> 'field' = 'seller_account_id';
