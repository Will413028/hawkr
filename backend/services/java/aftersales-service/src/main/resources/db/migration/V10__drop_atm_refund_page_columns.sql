-- ATM 退款頁（PayUni 頁面版退款）隨 PayUni 金流刪除：退款只經 payment 的 CreateRefund（Stripe）。
-- 只有頁面版退款讀寫的欄位一併刪除；provider 回應的狀態與訊息仍由 RefundTaskWorker 寫入，改成與金流商無關的名稱。
ALTER TABLE refund_tasks
    DROP COLUMN mer_id,
    DROP COLUMN estimated_refund_date,
    DROP COLUMN bank_date,
    DROP COLUMN client_type,
    DROP COLUMN app_type,
    DROP COLUMN current_attempt;

ALTER TABLE refund_tasks RENAME COLUMN payuni_status TO provider_status;
ALTER TABLE refund_tasks RENAME COLUMN payuni_message TO provider_message;
