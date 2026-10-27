-- name: InsertPayuniStatusHistory :exec
INSERT INTO payuni_status_history (
  id, account_id, store_id, payuni_mer_id, source,
  mer_status, member_status, credit_status,
  applepay_status, googlepay_status, atm_status,
  reason, raw_response, recorded_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
);

-- name: ListPayuniStatusHistory :many
SELECT * FROM payuni_status_history
WHERE account_id = $1
ORDER BY recorded_at DESC
LIMIT $2;

-- name: DeletePayuniStatusHistoryOlderThan :execrows
DELETE FROM payuni_status_history WHERE recorded_at < $1;
