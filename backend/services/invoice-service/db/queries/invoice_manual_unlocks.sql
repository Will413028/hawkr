
-- name: ExpireStaleManualUnlocks :execrows
UPDATE invoice_manual_unlocks
SET status = 'EXPIRED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE operation_id = sqlc.arg(operation_id)
  AND status = 'AWAITING_SECOND_APPROVAL'
  AND expires_at <= sqlc.arg(now);

-- name: InsertManualUnlock :one
INSERT INTO invoice_manual_unlocks (
  id, operation_id, status, requested_by_account_id, request_reason,
  provider_check_evidence, provider_checked_at, requested_at, expires_at,
  cancel_evidence_kind, cancel_evidence_object_key,
  version, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(operation_id), 'AWAITING_SECOND_APPROVAL',
  sqlc.arg(requested_by_account_id), sqlc.arg(request_reason),
  sqlc.arg(provider_check_evidence), sqlc.arg(provider_checked_at),
  sqlc.arg(now), sqlc.arg(expires_at),
  sqlc.narg(cancel_evidence_kind), sqlc.narg(cancel_evidence_object_key),
  0, sqlc.arg(now), sqlc.arg(now))
RETURNING *;

-- name: GetManualUnlockByID :one
SELECT * FROM invoice_manual_unlocks WHERE id = sqlc.arg(id);

-- name: GetOpenManualUnlockByOperation :one
SELECT * FROM invoice_manual_unlocks
WHERE operation_id = sqlc.arg(operation_id)
  AND status = 'AWAITING_SECOND_APPROVAL'
  AND expires_at > sqlc.arg(now);

-- name: GetLatestManualUnlockByOperation :one
SELECT * FROM invoice_manual_unlocks
WHERE operation_id = sqlc.arg(operation_id)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: DecideManualUnlock :execrows
UPDATE invoice_manual_unlocks
SET status = sqlc.arg(next_status),
    approved_by_account_id = sqlc.arg(approved_by_account_id),
    decision_reason = sqlc.narg(decision_reason),
    decided_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND operation_id = sqlc.arg(operation_id)
  AND status = 'AWAITING_SECOND_APPROVAL'
  AND expires_at > sqlc.arg(now)
  AND requested_by_account_id <> sqlc.arg(approved_by_account_id)
  AND version = sqlc.arg(expected_version);
