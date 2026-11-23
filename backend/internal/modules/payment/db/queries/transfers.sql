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
-- 一筆付款最多一筆有效（PENDING、SUCCEEDED）轉帳，由唯一索引 transfers_active_payment 守：已有有效轉帳時不寫（ON CONFLICT DO NOTHING），呼叫端再讀既有的那筆。
INSERT INTO payment.transfers (id, payment_id, store_id, amount, destination_account_id, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(payment_id), sqlc.arg(store_id), sqlc.arg(amount), sqlc.arg(destination_account_id), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (payment_id) WHERE status IN ('PENDING', 'SUCCEEDED') DO NOTHING;

-- name: ActiveTransferByPayment :one
SELECT *
FROM payment.transfers
WHERE payment_id = $1 AND status IN ('PENDING', 'SUCCEEDED');

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
-- 呼叫 provider 之前先記「已呼叫」的時刻（已記過的保留最早的）：之後列上就分得出「從沒呼叫」與「呼叫過、結果不明」。只有 PENDING 的列寫。
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

-- name: AddTransferReversal :one
-- 累加這次撤回的額度（同 transaction 已鎖轉帳列；超過 amount 時 CHECK transfers_reversed_check 擋下）。
UPDATE payment.transfers
SET reversed_amount = reversed_amount + sqlc.arg(amount),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: PendingTransferIDs :many
-- 週期掃描：PENDING、閒置超過門檻的轉帳（job 掉了的保底；每次呼叫都會更新 updated_at，重試中的不會被重複排）。
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
-- 撤回 provider 之前先記「已呼叫」的時刻（已記過的保留最早的）。只有 REVERSING 的列寫。
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
-- 撤回的終局失敗（provider 永久拒絕，或結果不明超過期限）：退款進 FAILED，不佔額度。
UPDATE payment.refunds
SET status = 'FAILED',
    failure_reason = sqlc.arg(failure_reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND status = 'REVERSING'
RETURNING *;

-- name: ReversingRefundIDs :many
-- 週期掃描：REVERSING、閒置超過門檻的退款。
SELECT id
FROM payment.refunds
WHERE status = 'REVERSING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);
