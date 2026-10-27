-- name: InsertMerchantVerificationOutbox :one
INSERT INTO merchant_verification_outbox (id, account_id, event_type, reason, payload, status, attempt_count, next_attempt_at, created_at)
VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, $7) RETURNING *;

-- name: FetchPendingMerchantVerificationOutbox :many
-- 刻意沒有 FOR UPDATE SKIP LOCKED：呼叫端在自動提交下跑，SELECT 一回傳鎖就釋放，
-- 搶佔改由 ClaimMerchantVerificationOutbox 的 lease CAS 負責（《多副本排程搶佔規範》）。
SELECT * FROM merchant_verification_outbox
WHERE status = 'pending' AND next_attempt_at <= $1
ORDER BY next_attempt_at LIMIT $2;

-- name: ClaimMerchantVerificationOutbox :execrows
UPDATE merchant_verification_outbox
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdateMerchantVerificationOutboxAttempt :exec
UPDATE merchant_verification_outbox
SET attempt_count = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1;

-- name: MarkMerchantVerificationOutboxDone :exec
UPDATE merchant_verification_outbox SET status = 'done', completed_at = $2 WHERE id = $1;

-- name: MarkMerchantVerificationOutboxFailed :exec
UPDATE merchant_verification_outbox SET status = 'failed', last_error = $2, completed_at = $3 WHERE id = $1;

-- name: CountPendingMerchantVerificationOutbox :one
SELECT COUNT(*) FROM merchant_verification_outbox WHERE status = 'pending';

-- name: OldestPendingMerchantVerificationOutboxCreatedAt :one
SELECT created_at FROM merchant_verification_outbox
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteDoneMerchantVerificationOutboxBefore :execrows
DELETE FROM merchant_verification_outbox
WHERE id IN (
  SELECT sub.id FROM merchant_verification_outbox AS sub
  WHERE sub.status = 'done' AND sub.completed_at < sqlc.arg(cutoff)
  LIMIT sqlc.arg(row_limit)
);
