-- 啟用 `invoice.allowance.issued.buyer`（P4）—— 20260830160000 那批 13 列的最後一個停用項。
--
-- 前一次解除（20260903020000）的檔尾把它單獨留下，理由逐字是：
--   「`AdjustmentRepository` 只有 Insert 與三個唯讀方法，**沒有任何狀態轉換** ⇒
--     折讓的執行面（送 AMEGO、確認、狀態機）整個不存在，缺的是功能本身不是 publisher。」
-- 那句話從本波起不成立：折讓執行軸已落地（#3073）。
--
-- 🔓 20260830160000 檔頭寫死的解除條件，本波逐項達成：
--   ① routing key 在 invoice-service 出現 outbox Insert 呼叫點：
--      `internal/usecase/allowance.go` 的 `settleAllowanceCompleted`，掛在
--      「adjustment 轉 COMPLETED ＋ 累計落帳」的**同一個交易**內。
--      兩條路都會走到它：G09（送件當下就拿到成功）與 G13（結果未明後查詢確認）。
--   ② `tests/contract/golden/event_payload_keys.json` 補上 emitted_keys，
--      逐欄抄自 `allowanceIssuedEvent` 的 json tag。
--   ③ 從 `tests/contract/golden/known_pending.json` 移除該列
--      ——移除後該檔的 invoice 條目歸零。
--
-- ⚠️ topology 零改動：`notification.invoice` 以 `'#'` 綁定 `hawkr.invoice`。
--
-- 為什麼這一則非發不可：折讓**不會**改變發票狀態（原票仍是 ISSUED），
-- 所以買家的發票中心從外觀上看不出任何變化——退了錢、開了折讓單，而他手上
-- 那張發票長得跟昨天一模一樣。少了這則通知，「錢退了但憑證怎麼算」對買家是全黑的。
--
-- ⚠️ `cumulative_allowance_amount` 送的是**本次落帳後**的累計
-- （`doc.AllowanceTotalAmount + gross`），不是折讓前的舊值：payload 在同一交易內
-- 於 AddAllowanceTotal **之前**組好，此時 doc 上那一欄還是舊的。
UPDATE "notification_templates"
   SET "enabled" = true
 WHERE "exchange" = 'hawkr.invoice'
   AND "routing_key" = 'invoice.allowance.issued'
   AND "event_type" = 'invoice.allowance.issued.buyer'
   AND "target_user_type" = 'buyer';
