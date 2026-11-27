
-- name: InsertInvoiceProviderAuditLog :exec
INSERT INTO invoice_provider_audit_logs (
  operation_id, issuer_id, provider, endpoint, attempt_seq,
  request_masked, response_masked, raw_body_hash,
  http_status, provider_code, provider_message, latency_ms, error_kind, correlation_id, occurred_at)
VALUES (
  sqlc.narg(operation_id), sqlc.arg(issuer_id), sqlc.arg(provider), sqlc.arg(endpoint),
  sqlc.arg(attempt_seq), sqlc.arg(request_masked), sqlc.narg(response_masked), sqlc.arg(raw_body_hash),
  sqlc.narg(http_status), sqlc.narg(provider_code), sqlc.narg(provider_message), sqlc.narg(latency_ms),
  sqlc.narg(error_kind), sqlc.narg(correlation_id), sqlc.arg(now));

-- name: ListInvoiceProviderAuditLogsByOperation :many
SELECT * FROM invoice_provider_audit_logs
WHERE operation_id = sqlc.arg(operation_id)
ORDER BY attempt_seq;

-- name: GetLatestProviderAuditCorrelationByIssuer :one
SELECT correlation_id FROM invoice_provider_audit_logs
WHERE issuer_id = sqlc.arg(issuer_id)
  AND correlation_id IS NOT NULL
ORDER BY occurred_at DESC, id DESC
LIMIT 1;
