-- Issue #686：賣家收到的評價通知（review.created）顯示具體買家而非「買家」通用字。
-- review-service 在 ReviewCreatedEvent payload 補 buyerNickname / buyerAvatar；本檔擴充
-- 賣家端 template 的 data_fields，讓 notification-service 把這兩個欄位塞進 inbox.data，
-- 前端 helper (_personalize) 抓到 buyerNickname 後將「買家」替換成具體名稱。
--
-- 取不到 buyer info 時 publisher 會塞空字串，FieldSpec.Resolve 不影響其他欄位；
-- 前端 fallback 到原文「買家對訂單⋯」。

UPDATE "notification_templates"
SET "data_fields" = '{
  "refType":       {"const": "ORDER"},
  "refId":         {"field": "orderNo"},
  "order_id":      {"field": "orderNo"},
  "review_id":     {"field": "reviewId"},
  "buyerNickname": {"field": "buyerNickname"},
  "buyerAvatar":   {"field": "buyerAvatar"}
}'::jsonb
WHERE "event_type" = 'review.created' AND "target_user_type" = 'seller';
