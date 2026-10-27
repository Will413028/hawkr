-- 49 §6.3/S3 批 2：order-service MQ payload sellerAvatar → sellerAvatarKey（seller_avatar → seller_avatar_key）。
-- 消費端同 PR 群改造（§6.7 S3）：order.paid / order.cancelled 系列 template 的 image_key_template
-- 佔位由 #{seller_avatar} 改讀 #{seller_avatar_key}，否則 payload key 對不上 → FCM 靜默無圖（§23 教訓）。
-- 僅動 order 來源事件（routing_key order.paid / order.cancelled）；shipping 出貨事件 seller_avatar
-- 屬 S9 cascade 隨批 3（seller）改，故 shipment.* template 維持 #{seller_avatar} 不動。
-- worker 續於 push 前 mediaurl.BuildURL 組 URL（S5），值為 key 或 http passthrough 皆可。
UPDATE "notification_templates"
SET "image_key_template" = '#{seller_avatar_key}'
WHERE "image_key_template" = '#{seller_avatar}'
  AND "routing_key" IN ('order.paid', 'order.cancelled');
