
-- name: InsertInvoiceStatusHistory :exec
INSERT INTO invoice_status_history (
  subject_type, subject_id, from_status, to_status, reason_code, reason_detail,
  actor_type, actor_account_id, correlation_id, occurred_at)
VALUES (
  sqlc.arg(subject_type), sqlc.arg(subject_id), sqlc.narg(from_status), sqlc.arg(to_status),
  sqlc.narg(reason_code), sqlc.narg(reason_detail),
  sqlc.arg(actor_type), sqlc.narg(actor_account_id), sqlc.narg(correlation_id), sqlc.arg(now));

-- name: ListInvoiceStatusHistory :many
SELECT * FROM invoice_status_history
WHERE subject_type = sqlc.arg(subject_type) AND subject_id = sqlc.arg(subject_id)
ORDER BY occurred_at;
