-- 啟用 `invoice.voided.buyer`（P3）與 `invoice.operation.review_required.seller`（P7）。
--
-- 兩列自 20260830160000 起以 `enabled = false` 存在，停用理由逐字寫在該檔檔頭的 (A)：
-- 「publisher 根本還沒實作」。#3055 修好 invoice.reissued 之後做的橫向稽查發現，
-- 那一批 5 個裡有 3 個屬於「功能已完成、只漏 publisher」——reissued 已於
-- 20260903000000 解除，這裡是剩下的兩個。
--
-- 🔓 該檔寫死的解除條件，本波逐項達成：
--   ① routing key 在 invoice-service 出現 outbox Insert 呼叫點：
--      P3 ⇒ `internal/usecase/void.go` 的 `emitVoided`，掛在 `ConfirmVoid` 把原票轉
--           VOIDED 的**同一個交易**內。
--      P7 ⇒ `internal/usecase/issuance.go` 的 `emitReviewRequired`，兩條路共用：
--           建單時的 fail-closed 落點（`persist`）與收據線的
--           `settleReceiptAwaitingManual`，都在各自的交易內。
--   ② `tests/contract/golden/event_payload_keys.json` 補上兩列 emitted_keys，
--      逐欄抄自 publisher struct（`voidedEvent`／`reviewRequiredEvent`）的 json tag。
--   ③ 從 `tests/contract/golden/known_pending.json` 移除這兩列。
--
-- ⚠️ topology 零改動：`notification.invoice` 以 `'#'` 綁定 `hawkr.invoice`。
--
-- 為什麼這兩則非發不可：
--   P3 —— 作廢**可以單獨發生**（退款折讓路徑就不重開）。少了它，買家手上那張憑證
--         失效了卻沒有任何人告訴他；作廢重開的情境下他也只會收到「已開立」，
--         不知道前一張已經作廢。
--   P7 —— fail-closed 的落點對賣家是完全無聲的：他只會發現發票一直沒開，
--         而系統其實已經把它掛在 ADM 的人工佇列上等人處理。
--
-- ⚠️ P10（`invoice.complaint.overdue`）同批一起解除：`MarkOverdue` 這個 repository
--    方法早就實作好了，缺的只是呼叫它的執行者。K05 併進 INV.dl 的**同一個 tick**
--    （`DeadlineWorker.RunOnce`）——另開排程器會在同名 metrics 重複註冊時 boot 期 panic，
--    那條約束逐字寫在 deadline_worker.go 的檔頭。
--    publisher 是 `usecase/complaint.go` 的 `SweepOverdueComplaints`，標記與事件同交易。
--
-- ⚠️ 只剩 `invoice.allowance.issued` 留在 known_pending.json，而它**不是**同一個形狀：
--    `AdjustmentRepository` 只有 Insert 與三個唯讀方法，**沒有任何狀態轉換** ⇒
--    折讓的執行面（送 AMEGO、確認、狀態機）整個不存在，缺的是功能本身不是 publisher。
--    58 §5.1.7／§5A.9 有完整定義，它是一個獨立的功能開發。
UPDATE "notification_templates"
   SET "enabled" = true
 WHERE "exchange" = 'hawkr.invoice'
   AND (("routing_key" = 'invoice.complaint.overdue'
         AND "event_type" = 'invoice.complaint.overdue.seller'
         AND "target_user_type" = 'seller')
     OR ("routing_key" = 'invoice.voided'
         AND "event_type" = 'invoice.voided.buyer'
         AND "target_user_type" = 'buyer')
     OR ("routing_key" = 'invoice.operation.review_required'
         AND "event_type" = 'invoice.operation.review_required.seller'
         AND "target_user_type" = 'seller'));
