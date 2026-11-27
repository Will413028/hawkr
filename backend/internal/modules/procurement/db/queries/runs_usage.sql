-- name: InsertRun :execrows
INSERT INTO procurement.agent_runs (id, buyer_id, request_id, conversation_id, interrupt_id, candidate_set_id, run_type, status, client_request_id, quota_period_start, created_at, updated_at)
VALUES (@id, @buyer_id, @request_id, @conversation_id, sqlc.narg('interrupt_id'), sqlc.narg('candidate_set_id'), @run_type, 'QUEUED', @client_request_id, sqlc.narg('quota_period_start'), @now, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetRunForBuyer :one
SELECT * FROM procurement.agent_runs WHERE id = @id AND buyer_id = @buyer_id;

-- name: InsertRunInput :exec
INSERT INTO procurement.agent_run_inputs (run_id, buyer_message, option_id, locale, created_at) VALUES (@run_id, @buyer_message, sqlc.narg('option_id'), @locale, @now);

-- name: GetRunInput :one
SELECT i.* FROM procurement.agent_run_inputs i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.run_id = @run_id AND r.buyer_id = @buyer_id;

-- name: ConsumeUsage :one
INSERT INTO procurement.usage (buyer_id, period_start, used, updated_at)
SELECT @buyer_id, @period_start, 1, @now::timestamptz WHERE @quota_limit::int > 0
ON CONFLICT (buyer_id, period_start) DO UPDATE
SET used = procurement.usage.used + 1, updated_at = EXCLUDED.updated_at
WHERE procurement.usage.used < @quota_limit::int
RETURNING used;

-- name: RefundUsage :execrows
UPDATE procurement.usage SET used = used - 1, updated_at = @now
WHERE buyer_id = @buyer_id AND period_start = @period_start AND used > 0;

-- name: GetUsage :one
SELECT used FROM procurement.usage WHERE buyer_id = @buyer_id AND period_start = @period_start;
