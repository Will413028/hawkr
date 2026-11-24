-- name: MarkRefundsDeducted :exec
-- 轉帳 claim 算定金額時（呼叫端已鎖付款列與轉帳列）依當下的退款重寫「已算進轉帳金額」：非 FAILED 的退款為真，FAILED 的為假（算定時它已不在金額裡，不必補）。
UPDATE payment.refunds
SET transfer_deducted = (status <> 'FAILED')
WHERE payment_id = $1 AND transfer_deducted <> (status <> 'FAILED');

-- name: ClearRefundsDeducted :exec
-- 轉帳終局失敗（沒有搬錢）：沒有任何退款算進一筆有效轉帳的金額。呼叫端已鎖付款列與轉帳列。
UPDATE payment.refunds
SET transfer_deducted = false
WHERE payment_id = $1 AND transfer_deducted;

-- name: WaitingTopups :many
-- 付款底下補差額在 WAITING 的退款（已失敗、轉帳金額含它），依 ID 升序鎖。呼叫端已鎖付款列與轉帳列。
SELECT *
FROM payment.refunds
WHERE payment_id = $1 AND topup_status = 'WAITING'
ORDER BY id
FOR UPDATE;

-- name: StartRefundTopup :one
-- 開始補差額：WAITING → SENDING（只有已失敗的退款才有 WAITING，schema 的 refunds_topup_only_failed_check 也守）。
UPDATE payment.refunds
SET topup_status = 'SENDING',
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'WAITING'
RETURNING *;

-- name: ReleaseRefundTopup :one
-- 不補了：WAITING（轉帳失敗，或重新算定的金額已不含這筆退款）或 SENDING（補差額永久被拒絕）進 RELEASED，原因記在 topup_reason。
UPDATE payment.refunds
SET topup_status = 'RELEASED',
    topup_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = ANY(sqlc.arg(from_statuses)::text[])
RETURNING *;

-- name: MarkRefundTopupCalled :one
-- 補差額 provider 之前先記「已呼叫」的時刻（這把鍵已記過的保留最早的）。只有 SENDING 的列寫。
UPDATE payment.refunds
SET topup_called_at = COALESCE(topup_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING *;

-- name: ApplyRefundTopup :one
-- 補差額成功：SENDING → DONE 並記 provider 的轉帳 ID。
UPDATE payment.refunds
SET topup_status = 'DONE',
    topup_external_id = sqlc.arg(topup_external_id),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING *;

-- name: RotateRefundTopupKey :one
-- 補差額被 provider 已知拒絕（可重試、沒有執行），或查回證明上一次不存在又過了最短等待：換新的冪等鍵（也是新的嘗試 ID），清掉呼叫時刻與查回失敗次數，不計任何期限。
UPDATE payment.refunds
SET topup_idempotency_key = gen_random_uuid(),
    topup_called_at = NULL,
    topup_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING' AND topup_idempotency_key = sqlc.arg(old_key)
RETURNING *;

-- name: BumpRefundTopupLookupFailures :one
UPDATE payment.refunds
SET topup_lookup_failures = topup_lookup_failures + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING topup_lookup_failures;

-- name: ClearRefundTopupLookupFailures :exec
UPDATE payment.refunds
SET topup_lookup_failures = 0
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING' AND topup_lookup_failures > 0;

-- name: AnomalyRefundTopup :one
-- 補差額與 provider 不符、前置條件不成立或查回連續失敗：SENDING 進 ANOMALY（補掃不再重試），原因記在 topup_reason。
UPDATE payment.refunds
SET topup_status = 'ANOMALY',
    topup_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'SENDING'
RETURNING *;

-- name: ResolveRefundTopupDone :one
-- 後台人工結案（補差額的 ANOMALY）：provider 確實建立了補差額轉帳（topup_external_id 是證據），進 DONE。
UPDATE payment.refunds
SET topup_status = 'DONE',
    topup_external_id = sqlc.arg(topup_external_id),
    topup_called_at = COALESCE(topup_called_at, sqlc.arg(now)::timestamptz),
    topup_reason = NULL,
    topup_lookup_failures = 0,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'ANOMALY'
RETURNING *;

-- name: ResolveRefundTopupReleased :one
-- 後台人工結案（補差額的 ANOMALY）：確認 provider 沒有補差額轉帳，不補了（人工另行處理）。
UPDATE payment.refunds
SET topup_status = 'RELEASED',
    topup_reason = sqlc.arg(reason),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND topup_status = 'ANOMALY'
RETURNING *;

-- name: SendingTopupRefundIDs :many
-- 週期掃描：補差額 SENDING、閒置超過門檻的退款（ANOMALY 不在內，partial index refunds_topup_sending）。
SELECT id
FROM payment.refunds
WHERE topup_status = 'SENDING' AND updated_at <= sqlc.arg(idle_before) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(batch);
