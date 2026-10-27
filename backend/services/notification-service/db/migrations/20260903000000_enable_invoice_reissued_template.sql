-- 啟用 `invoice.reissued.buyer` 模板（backend#3055）。
--
-- 這一列自 20260830160000 起就以 `enabled = false` 存在，停用理由逐字寫在該檔檔頭的
-- (A)：「publisher 根本還沒實作」——實查當時 invoice-service 全域對
-- RoutingKeyInvoiceReissued 零使用點。
--
-- 🔓 該檔寫死的解除條件，本波逐項達成：
--   ① routing key 在 invoice-service 出現 outbox Insert 呼叫點
--      ⇒ `internal/usecase/submit.go` 的 `completeReissueIfAny`（原票轉 SUPERSEDED
--        之後、同一個交易內）。
--   ② `tests/contract/golden/event_payload_keys.json` 補上 `invoice.reissued.buyer`
--      的 emitted_keys，逐欄抄自 publisher struct `reissuedEvent` 的 json tag
--      （notification_ref_id／invoice_document_id／order_no／buyer_id／store_id／
--        voided_document_id／voided_invoice_no／new_invoice_no／occurred_at
--        ＋ 派送時補的 notify_seller_account_id）。
--      涵蓋本模板用到的每一個 `#{...}` 與 data_fields[*].field。
--   ③ 從 `tests/contract/golden/known_pending.json` 移除該列。
--
-- ⚠️ topology 零改動：`notification.invoice` 以 `'#'` 綁定 `hawkr.invoice`
--    （infra/mq/topology/topology.yaml），新 routing key 天然被涵蓋。
--
-- 為什麼這則通知非發不可：重開票走的是正常開票路徑，那條路已經發過一則
-- INVOICE_ISSUED ⇒ 少了本事件，「這張是取代前一張的」在事件面完全不可區分。
-- 買家會收到兩則「發票已開立」而不知道第一張已經作廢。
UPDATE "notification_templates"
   SET "enabled" = true
 WHERE "exchange" = 'hawkr.invoice'
   AND "routing_key" = 'invoice.reissued'
   AND "event_type" = 'invoice.reissued.buyer'
   AND "target_user_type" = 'buyer';
