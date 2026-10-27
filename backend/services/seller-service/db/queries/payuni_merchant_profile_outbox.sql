-- name: InsertPayuniMerchantProfileOutbox :one
INSERT INTO payuni_merchant_profile_outbox (id, store_id, account_id, payload, status, attempt_count, next_attempt_at, created_at)
VALUES ($1, $2, $3, $4, 'pending', 0, $5, $6) RETURNING *;

-- name: FetchPendingPayuniMerchantProfileOutbox :many
-- 刻意沒有 FOR UPDATE SKIP LOCKED：呼叫端在自動提交下跑，SELECT 一回傳鎖就釋放，
-- 搶佔改由 ClaimPayuniMerchantProfileOutbox 的 lease CAS 負責（《多副本排程搶佔規範》）。
SELECT * FROM payuni_merchant_profile_outbox
WHERE status = 'pending' AND next_attempt_at <= $1
ORDER BY next_attempt_at LIMIT $2;

-- name: ClaimPayuniMerchantProfileOutbox :execrows
UPDATE payuni_merchant_profile_outbox
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdatePayuniMerchantProfileOutboxAttempt :exec
UPDATE payuni_merchant_profile_outbox
SET attempt_count = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1;

-- name: MarkPayuniMerchantProfileOutboxDone :exec
UPDATE payuni_merchant_profile_outbox SET status = 'done', completed_at = $2 WHERE id = $1;

-- name: MarkPayuniMerchantProfileOutboxFailed :exec
UPDATE payuni_merchant_profile_outbox SET status = 'failed', last_error = $2, completed_at = $3 WHERE id = $1;

-- name: CountPendingPayuniMerchantProfileOutbox :one
SELECT COUNT(*) FROM payuni_merchant_profile_outbox WHERE status = 'pending';

-- name: OldestPendingPayuniMerchantProfileOutboxCreatedAt :one
SELECT created_at FROM payuni_merchant_profile_outbox
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteDonePayuniMerchantProfileOutboxBefore :execrows
DELETE FROM payuni_merchant_profile_outbox
WHERE id IN (
  SELECT sub.id FROM payuni_merchant_profile_outbox AS sub
  WHERE sub.status = 'done' AND sub.completed_at < sqlc.arg(cutoff)
  LIMIT sqlc.arg(row_limit)
);
