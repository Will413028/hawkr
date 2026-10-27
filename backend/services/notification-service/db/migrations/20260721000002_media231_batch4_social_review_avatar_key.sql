-- 49 §6.3/S3 批 4：social（post.created）＋review（review.created）MQ payload avatar → *Key。
-- (1) PostCreatedEvent.SellerAvatar → SellerAvatarKey（seller_avatar → seller_avatar_key）：
--     post.created（follower 買家）template image_key_template 佔位由 #{seller_avatar} 改讀 #{seller_avatar_key}。
--     批 2 已把 order 來源事件、批 3 已把 shipping 來源事件的 #{seller_avatar} 轉走，故此時
--     殘存於 routing_key='post.created' 的 snake '#{seller_avatar}' 唯一來源即 social。
-- (2) ReviewCreatedEvent.BuyerAvatar → BuyerAvatarKey（buyer_avatar → buyer_avatar_key）：
--     review.created（賣家）template data_fields buyerAvatar.field 由 buyer_avatar 改讀 buyer_avatar_key。
--     批 3 shipment.created 的 buyerAvatar 已切；本檔僅限 event_type='review.created'。
-- worker 續於 push 前 mediaurl.BuildURL 組 URL（S5），值為 objectKey 或 http passthrough 皆可。
UPDATE "notification_templates"
SET "image_key_template" = '#{seller_avatar_key}'
WHERE "image_key_template" = '#{seller_avatar}'
  AND "routing_key" = 'post.created';

UPDATE "notification_templates"
SET "data_fields" = jsonb_set("data_fields", '{buyerAvatar,field}', '"buyer_avatar_key"')
WHERE "event_type" = 'review.created'
  AND "target_user_type" = 'seller'
  AND "data_fields" -> 'buyerAvatar' ->> 'field' = 'buyer_avatar';
