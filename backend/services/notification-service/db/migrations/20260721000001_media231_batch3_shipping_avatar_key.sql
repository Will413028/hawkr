-- 49 §6.3/S3 批 3：shipping-service MQ payload avatar → *Key（S9 cascade 自 seller/buyer）。
-- (1) ShippingEventPayload.sellerAvatar → sellerAvatarKey（seller_avatar → seller_avatar_key）：
--     shipment.delivered / shipment.status.changed（含退貨變體）template image_key_template 佔位改讀。
--     批 2 已把 order 來源事件的 #{seller_avatar} 轉為 #{seller_avatar_key}，故此時殘存的
--     snake '#{seller_avatar}' 皆為 shipping 來源；post.created / refund.succeeded 用駝峰
--     '#{sellerAvatar}'（社群批 4 / 金流），不受此更新影響。
-- (2) ShipmentCreatedPayload.buyerAvatar → buyerAvatarKey（buyer_avatar → buyer_avatar_key）：
--     shipment.created（買家已下單）template data_fields buyerAvatar.field 改讀。
--     review.created 的 buyer_avatar 屬 review-service（批 4），本檔不動（僅限 routing_key=shipment.created）。
-- worker 續於 push 前 mediaurl.BuildURL 組 URL（S5），值為 objectKey 或 http passthrough 皆可。

UPDATE "notification_templates"
SET "image_key_template" = '#{seller_avatar_key}'
WHERE "image_key_template" = '#{seller_avatar}'
  AND ("routing_key" LIKE 'shipment%' OR "routing_key" LIKE 'return_shipment%');

UPDATE "notification_templates"
SET "data_fields" = jsonb_set("data_fields", '{buyerAvatar,field}', '"buyer_avatar_key"')
WHERE "routing_key" = 'shipment.created'
  AND "data_fields" -> 'buyerAvatar' ->> 'field' = 'buyer_avatar';
