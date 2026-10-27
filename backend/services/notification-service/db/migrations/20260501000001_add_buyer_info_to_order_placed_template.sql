-- Issue #686：賣家收到的訂單通知（order.placed）顯示具體買家而非「買家」通用字。
-- 後端在 OrderPaidPayload 加 buyerNickname / buyerAvatar；本檔擴充賣家端 template 的
-- data_fields，讓 notification-service 把這兩個欄位塞進 inbox.data，前端 helper
-- (_personalize) 抓到 buyerNickname 後將 title / body 的「買家」替換成具體名稱。
--
-- 取不到 buyer info 時 publisher 端會塞 null，FieldSpec.Resolve 會略過該欄位
-- (template_handler.go L320-324)，前端 fallback 到原文「買家已下單」。

UPDATE "notification_templates"
SET "data_fields" = '{
  "refType":       {"const": "ORDER"},
  "refId":         {"field": "orderNo"},
  "order_id":      {"field": "orderNo"},
  "buyerNickname": {"field": "buyerNickname"},
  "buyerAvatar":   {"field": "buyerAvatar"}
}'::jsonb
WHERE "event_type" = 'order.placed' AND "target_user_type" = 'seller';
