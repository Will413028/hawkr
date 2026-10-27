-- issue #1176 PR-3: flipped 事件 notification template read-key camelCase→snake
-- 對齊已 flip 的 Go/Java publisher（review/post/follow/product.viewed/favorite/restock/coupon.issued/merchant/chat/shipment.anomaly）。
-- data_fields 左鍵(FCM 契約)保 camelCase；只改右側 field 值、target_user_field、#{placeholder}。

UPDATE notification_templates SET
  target_user_field = 'buyer_id',
  body_template = '你收到了「#{coupon_name}」優惠券',
  image_url_template = '#{coupon_image_url}',
  reference_id_template = 'coupon_#{buyer_id}_#{coupon_id}',
  data_fields = '{"refId": {"field": "coupon_id"}, "refType": {"const": "COUPON"}, "couponId": {"field": "coupon_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'coupon.issued' AND target_user_type = 'buyer';

UPDATE notification_templates SET
  target_user_field = 'follower_id',
  title_template = '#{seller_name} 發布了新貼文',
  body_template = '#{content_preview}',
  image_url_template = '#{seller_avatar}',
  reference_id_template = 'post_#{post_id}_#{follower_id}',
  data_fields = '{"refId": {"field": "post_id"}, "postId": {"field": "post_id"}, "refType": {"const": "POST"}, "sellerId": {"field": "seller_account_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'post.created' AND target_user_type = 'buyer';

UPDATE notification_templates SET
  target_user_field = 'buyer_account_id',
  body_template = '「#{product_name}」已經補貨上架，快來搶購吧！',
  image_url_template = '#{image_url}',
  reference_id_template = 'restock_#{subscription_id}',
  data_fields = '{"refId": {"field": "product_id"}, "refType": {"const": "PRODUCT"}, "deep_link": {"field": "deep_link"}, "image_url": {"field": "image_url"}, "product_id": {"field": "product_id"}, "subscription_id": {"field": "subscription_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'product.restock.target' AND target_user_type = 'buyer';

UPDATE notification_templates SET
  target_user_field = 'seller_account_id',
  body_template = '#{buyer_name|有人} 收藏了您的「#{product_name}」！',
  reference_id_template = 'fav_#{seller_account_id}_#{product_id}',
  data_fields = '{"productId": {"field": "product_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'product.favorited' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'merchant_id',
  reference_id_template = 'merchant_verification_#{merchant_id}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'merchant.verification.approved' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'merchant_id',
  reference_id_template = 'merchant_verification_#{merchant_id}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'merchant.verification.rejected' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'merchant_id',
  reference_id_template = 'merchant_verification_#{merchant_id}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'merchant.verification.pending_info' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'seller_id',
  body_template = '有人正在看您的「#{product_name}」商品！把握機會主動聊聊，提升 30% 成交機率。',
  reference_id_template = 'viewed_#{seller_id}_#{product_id}',
  data_fields = '{"productId": {"field": "product_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'product.viewed' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'merchant_id',
  reference_id_template = 'merchant_verification_forwarded_#{merchant_id}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'merchant.verification.forwarded' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'merchant_id',
  body_template = '#{follower_name|有人} 開始追蹤了！',
  reference_id_template = 'follow_#{merchant_id}_#{follower_id}',
  data_fields = '{"followerId": {"field": "follower_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'follow.added' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'seller_id',
  body_template = '#{liker_name} 覺得您的貼文很讚！',
  reference_id_template = 'post_liked_#{seller_id}_#{post_id}',
  data_fields = '{"postId": {"field": "post_id"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'post.liked' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'seller_account_id',
  body_template = '買家對訂單 ##{order_no} 給了 #{rating} 星評價！',
  reference_id_template = 'review_#{order_no}_#{seller_account_id}',
  data_fields = '{"refId": {"field": "order_no"}, "orderId": {"field": "order_no"}, "refType": {"const": "ORDER"}, "reviewId": {"field": "review_id"}, "buyerAvatar": {"field": "buyer_avatar"}, "buyerNickname": {"field": "buyer_nickname"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'review.created' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'seller_account_id',
  body_template = '訂單 #{order_no} #{anomaly_type_label}，請儘速處理。',
  reference_id_template = '#{order_no}|#{anomaly_type}',
  data_fields = '{"refId": {"field": "order_no"}, "orderNo": {"field": "order_no"}, "refType": {"const": "SHIPMENT_ANOMALY"}, "severity": {"field": "severity"}, "anomalyType": {"field": "anomaly_type"}}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'shipment.anomaly.detected.seller' AND target_user_type = 'seller';

UPDATE notification_templates SET
  target_user_field = 'receiver_id',
  title_template = '#{sender_name}',
  body_template = '#{content_preview}',
  reference_id_template = '#{message_id}',
  data_fields = '{"refId": {"field": "conversation_id"}, "sentAt": {"field": "sent_at"}, "refType": {"const": "CHAT"}, "senderId": {"field": "sender_id"}, "messageId": {"field": "message_id"}, "senderName": {"field": "sender_name"}, "senderAvatar": {"field": "sender_avatar"}, "conversationId": {"field": "conversation_id"}}',
  payload_filter = '{"receiver_type": "buyer"}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'chat.message.sent' AND target_user_type = 'buyer';

UPDATE notification_templates SET
  target_user_field = 'receiver_id',
  title_template = '#{sender_name}',
  body_template = '#{content_preview}',
  reference_id_template = '#{message_id}',
  data_fields = '{"refId": {"field": "conversation_id"}, "sentAt": {"field": "sent_at"}, "refType": {"const": "CHAT"}, "senderId": {"field": "sender_id"}, "messageId": {"field": "message_id"}, "senderName": {"field": "sender_name"}, "senderAvatar": {"field": "sender_avatar"}, "conversationId": {"field": "conversation_id"}}',
  payload_filter = '{"receiver_type": "seller"}',
  updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'chat.message.sent' AND target_user_type = 'seller';
