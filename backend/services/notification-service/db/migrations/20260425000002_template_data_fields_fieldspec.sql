-- 將 notification_templates.data_fields 從舊 string 形式升級為新 FieldSpec 形式。
--
-- 舊格式（v1）：{fcm_key: payload_field_name}                   值型別 string
-- 新格式（v2）：{fcm_key: {const: literal} | {field: name}}     值型別 object
--
-- 轉換策略：
--   1) 把所有「值還是 string」的 row 整體轉為 {"field": <原字串>} 形式。
--   2) 訂單系列補上 refType / refId 欄位（對齊 #626 契約）。
--      data_fields 改寫為 {"refType": {"const": "ORDER"}, "refId": {"field": "orderNo"}, ...}。

-- Step 1：把所有 string-value rows 整批轉成 {"field": "..."} 形式。
-- 略過已經是 object 的 row（再跑一次也安全）。
UPDATE "notification_templates"
SET "data_fields" = (
  SELECT jsonb_object_agg(key, jsonb_build_object('field', value))
  FROM jsonb_each_text("data_fields")
)
WHERE "data_fields" IS NOT NULL
  AND EXISTS (
    SELECT 1
    FROM jsonb_each("data_fields") AS e
    WHERE jsonb_typeof(e.value) = 'string'
  );

-- Step 2：訂單系列補 refType / refId（對齊 #626：FCM data 含 refType="ORDER" + refId=orderNo）。
-- 保留 order_id 鍵以兼容 schema_url 模板 hawkr://orders/{order_id}。
UPDATE "notification_templates"
SET "data_fields" = '{
  "refType":  {"const": "ORDER"},
  "refId":    {"field": "orderNo"},
  "order_id": {"field": "orderNo"}
}'::jsonb
WHERE "event_type" IN (
  'order.paid',
  'order.placed',
  'order.shipped.buyer',
  'order.shipped.merchant',
  'order.completed.buyer',
  'order.completed.merchant',
  'order.cancelled.buyer',
  'order.cancelled.merchant',
  'order.refund.requested.buyer',
  'order.refund.requested.merchant',
  'shipment.delivered',
  'shipment.delivered.seller',
  'refund.completed',
  'payout.completed',
  'return.shipped'
);

-- Step 3：review.created 也屬訂單延伸（評價跳訂單詳情），補 refType / refId 並保留 review_id。
UPDATE "notification_templates"
SET "data_fields" = '{
  "refType":   {"const": "ORDER"},
  "refId":     {"field": "orderNo"},
  "order_id":  {"field": "orderNo"},
  "review_id": {"field": "reviewId"}
}'::jsonb
WHERE "event_type" = 'review.created';
