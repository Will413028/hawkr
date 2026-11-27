
-- name: ListInvoiceComplaintEvents :many
SELECT * FROM invoice_complaint_events
WHERE complaint_id = sqlc.arg(complaint_id)
ORDER BY occurred_at;

-- name: GetInvoiceComplaintEventByID :one
SELECT * FROM invoice_complaint_events WHERE id = sqlc.arg(id);

-- name: InsertInvoiceComplaintEvent :one
INSERT INTO invoice_complaint_events (
  complaint_id, event_type, action, actor_type, actor_account_id,
  message, attachment_object_keys, occurred_at)
VALUES (
  sqlc.arg(complaint_id), sqlc.arg(event_type), sqlc.narg(action),
  sqlc.arg(actor_type), sqlc.narg(actor_account_id),
  sqlc.narg(message), sqlc.arg(attachment_object_keys), sqlc.arg(now))
RETURNING *;
