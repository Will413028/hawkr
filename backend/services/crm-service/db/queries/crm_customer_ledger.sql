-- name: AppendLedgerEntry :execrows
INSERT INTO crm_customer_ledger (event_id, store_id, buyer_id, event_type, order_no, amount_delta, count_delta)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (event_id) DO NOTHING;

-- name: SumLedgerAmountForOrder :one
SELECT COALESCE(SUM(amount_delta), 0)::BIGINT AS total FROM crm_customer_ledger WHERE order_no = $1;

-- name: HasFullRefundEntry :one
SELECT EXISTS(SELECT 1 FROM crm_customer_ledger WHERE order_no = $1 AND event_type = 'refund_completed_full') AS has_full_refund;

-- name: LockOrderForNetting :exec
SELECT pg_advisory_xact_lock(hashtext($1));
