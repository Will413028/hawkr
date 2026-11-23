-- name: LockPaymentByOrderNo :one
-- 轉帳 use case 以訂單編號鎖付款列（付款列先於轉帳列與退款列）。
SELECT *
FROM payment.payments
WHERE order_no = $1
FOR UPDATE;

-- name: NonFailedRefundTotal :one
-- 付款已有的非失敗退款加總（進行中的也算）：轉帳金額是付款金額扣掉它。
SELECT COALESCE(SUM(amount), 0)::bigint AS total
FROM payment.refunds
WHERE payment_id = $1 AND status <> 'FAILED';

-- name: InsertTransfer :execrows
-- 一筆付款最多一筆有效（不是 FAILED 的）轉帳，只由唯一索引 transfers_active_payment 守（哪些狀態佔位只寫在索引的條件裡）：已有有效轉帳時不寫（ON CONFLICT DO NOTHING，不指定索引），呼叫端再讀既有的那筆。
INSERT INTO payment.transfers (id, payment_id, store_id, amount, destination_account_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_id), sqlc.arg(store_id), sqlc.arg(amount), sqlc.arg(destination_account_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING;

-- name: ActiveTransferByPayment :one
SELECT *
FROM payment.transfers
WHERE payment_id = $1 AND status <> 'FAILED';

-- name: SucceededTransferByPayment :one
SELECT *
FROM payment.transfers
WHERE payment_id = $1 AND status = 'SUCCEEDED';

-- name: TransferByID :one
SELECT *
FROM payment.transfers
WHERE id = $1;

-- name: LockTransfer :one
-- 鎖轉帳列序列化 reversed_amount 的累加（付款列先於轉帳列，轉帳列先於退款列）。
SELECT *
FROM payment.transfers
WHERE id = $1
FOR UPDATE;

-- name: MarkTransferCalled :one
-- 呼叫 provider 之前先記「已呼叫」的時刻（這把鍵已記過的保留最早的）：之後列上就分得出「從沒呼叫」與「呼叫過、結果不明」。只有 PENDING 的列寫。
UPDATE payment.transfers
SET called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: SucceedTransfer :one
UPDATE payment.transfers
SET status = 'SUCCEEDED',
    external_transfer_id = sqlc.arg(external_transfer_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: FailTransfer :one
UPDATE payment.transfers
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: RotateTransferKey :one
-- provider 已知拒絕（可重試、沒有執行）：換新的冪等鍵（也是新的嘗試 ID）、清掉這把鍵的呼叫時刻與查回失敗次數，不計任何期限。只有仍用 old_key 的 PENDING 列換（並行的 job 不重複換）。
UPDATE payment.transfers
SET idempotency_key = gen_random_uuid(),
    called_at = NULL,
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpTransferLookupFailures :one
-- 查回失敗一次：累計連續失敗次數（成功的查回清零）。
UPDATE payment.transfers
SET lookup_failures = lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING lookup_failures;

-- name: ClearTransferLookupFailures :exec
UPDATE payment.transfers
SET lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'PENDING' AND lookup_failures > 0;

-- name: AnomalyTransfer :one
-- provider 與本地紀錄不符、前置條件不成立或查回連續失敗：PENDING 進 ANOMALY（佔住唯一索引，補掃不再重試），原因記在 failure_reason。
UPDATE payment.transfers
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'PENDING'
RETURNING *;

-- name: ResolveTransferSucceeded :one
-- 後台人工結案：ANOMALY 的轉帳在 provider 確實存在（external_transfer_id 是證據），記成功並清掉原因。
UPDATE payment.transfers
SET status = 'SUCCEEDED',
    external_transfer_id = sqlc.arg(external_transfer_id),
    failure_reason = NULL,
    called_at = COALESCE(called_at, sqlc.arg(now)::timestamptz),
    lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: ResolveTransferFailed :one
-- 後台人工結案：確認 provider 沒有這筆轉帳，ANOMALY 進 FAILED（讓出唯一索引）。
UPDATE payment.transfers
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY'
RETURNING *;

-- name: AddTransferReversal :one
-- 累加這次撤回的額度（同 transaction 已鎖轉帳列；超過 amount 時 CHECK transfers_reversed_check 擋下）。
UPDATE payment.transfers
SET reversed_amount = reversed_amount + sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: PendingTransferIDs :many
-- 週期掃描：PENDING、閒置超過門檻的轉帳（job 掉了的保底；每次呼叫都會更新 updated_at，重試中的不會被重複排；ANOMALY 不在內，partial index transfers_pending）。
SELECT id
FROM payment.transfers
WHERE status = 'PENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);

-- name: RefundByID :one
SELECT *
FROM payment.refunds
WHERE id = $1;

-- name: LockRefund :one
SELECT *
FROM payment.refunds
WHERE id = $1
FOR UPDATE;

-- name: MarkRefundReversalCalled :one
-- 撤回 provider 之前先記「已呼叫」的時刻（這把鍵已記過的保留最早的）。只有 REVERSING 的列寫。
UPDATE payment.refunds
SET reversal_called_at = COALESCE(reversal_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ApplyRefundReversal :one
-- 撤回成功：REVERSING → REVERSED 並記 provider 的撤回 ID（與 transfers.reversed_amount 的累加在同一個 transaction）。
UPDATE payment.refunds
SET status = 'REVERSED',
    reversal_external_id = sqlc.arg(reversal_external_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: FailRefundReversal :one
-- 撤回的終局失敗（provider 永久拒絕，或查回證明上一次不存在又過了最短等待）：退款進 FAILED，不佔額度。
UPDATE payment.refunds
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: RotateRefundReversalKey :one
-- 撤回被 provider 已知拒絕（可重試、沒有執行）：換新的撤回冪等鍵（也是新的嘗試 ID），清掉呼叫時刻與查回失敗次數，不計任何期限。
UPDATE payment.refunds
SET reversal_idempotency_key = gen_random_uuid(),
    reversal_called_at = NULL,
    reversal_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING' AND reversal_idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpRefundReversalLookupFailures :one
UPDATE payment.refunds
SET reversal_lookup_failures = reversal_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING reversal_lookup_failures;

-- name: ClearRefundReversalLookupFailures :exec
UPDATE payment.refunds
SET reversal_lookup_failures = 0
WHERE id = sqlc.arg(id) AND status = 'REVERSING' AND reversal_lookup_failures > 0;

-- name: AnomalyRefundReversal :one
-- 撤回階段與 provider 不符、前置條件不成立或查回連續失敗：REVERSING 進 ANOMALY（仍佔額度，補掃不再重試），原因記在 failure_reason。
UPDATE payment.refunds
SET status = 'ANOMALY',
    failure_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ResolveRefundReversed :one
-- 後台人工結案（撤回階段的 ANOMALY）：provider 確實撤回了（reversal_external_id 是證據），退款進 REVERSED，之後照常往下。
UPDATE payment.refunds
SET status = 'REVERSED',
    reversal_external_id = sqlc.arg(reversal_external_id),
    reversal_called_at = COALESCE(reversal_called_at, sqlc.arg(now)::timestamptz),
    failure_reason = NULL,
    reversal_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY' AND refund_called_at IS NULL AND external_refund_id IS NULL
RETURNING *;

-- name: ResolveRefundFailed :one
-- 後台人工結案（撤回階段的 ANOMALY）：確認 provider 沒有撤回，退款進 FAILED（不佔額度）。
UPDATE payment.refunds
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'ANOMALY' AND refund_called_at IS NULL AND external_refund_id IS NULL
RETURNING *;

-- name: ReversingRefundIDs :many
-- 週期掃描：REVERSING、閒置超過門檻的退款（ANOMALY 不在內，partial index refunds_reversing）。
SELECT id
FROM payment.refunds
WHERE status = 'REVERSING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);
