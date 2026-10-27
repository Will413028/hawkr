-- 2026-06-25：賣家「商品被收藏」通知（product.favorited）綁在死 routing key。
-- 真實事件是 buyer-service（Go）發的 buyer.favorite.added（exchange hawkr.buyer），
-- 舊 binding exchange='hawkr.content' routing_key='product.favorited' 已無任何 publisher
-- → 賣家收不到收藏通知。
--
-- 同 PR 由 buyer-service 在 BuyerFavoriteEvent 補 enrich sellerAccountId / buyerName /
-- productName（camelCase，與本事件既有 productId/accountId 命名一致）。本 migration rebind
-- exchange/routing_key 並對齊欄位：target_user_field sellerId→sellerAccountId、
-- reference_id #{sellerId}→#{sellerAccountId}；buyerName best-effort，空時 per-field default 退「有人」。
-- data_fields（productId）與 event_type metadata 不動。
UPDATE notification_templates
SET exchange              = 'hawkr.buyer',
    routing_key           = 'buyer.favorite.added',
    target_user_field     = 'sellerAccountId',
    body_template         = '#{buyerName|有人} 收藏了您的「#{productName}」！',
    reference_id_template  = 'fav_#{sellerAccountId}_#{productId}'
WHERE routing_key = 'product.favorited'
  AND target_user_type = 'seller'
  AND exchange = 'hawkr.content';
