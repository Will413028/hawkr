-- issue #1514：賣家停權傳輸值統一改送 CodeMeta Value 數字碼（"POLICY_VIOLATION" → "9601"）。
-- 本模板引擎只做字面 `#{field}` 代入、無 lookup 能力，若不處理，切換後通知內文會從
-- 「原因代碼：POLICY_VIOLATION」變成更不可讀的「原因代碼：9601」。
--
-- seller-service 已在 seller.suspended MQ payload 新增 reasonLabel 欄位（人類可讀
-- label，例如「違反平台規範」），與既有 reasonCode 並存（reasonCode 格式不變，其他
-- consumer 如 im-service 仍讀 Name 字串）。本 migration 把模板改讀 reasonLabel。
UPDATE "notification_templates"
SET
  body_template = '停權原因：#{reasonLabel}。停權期間商品已暫時下架、未付款訂單已取消，如有疑問請至申訴頁提出申訴。',
  data_fields = '{"refType": {"const": "SELLER_SUSPENSION"}, "refId": {"field": "suspensionId"}, "reasonLabel": {"field": "reasonLabel"}}'
WHERE event_type = 'seller.suspended.notice';
