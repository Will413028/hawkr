-- name: InsertRun :execrows
-- 建 run 的方法的冪等鍵放在 run 上（S4）；同一位買家的同一個 client_request_id 已有 run 時回 0 列。形狀由 A9-3b 補全。
INSERT INTO procurement.agent_runs (id, buyer_id, request_id, conversation_id, run_type, status, client_request_id, quota_period_start, created_at, updated_at)
VALUES (@id, @buyer_id, @request_id, @conversation_id, @run_type, 'QUEUED', @client_request_id, @quota_period_start, @now, @now)
ON CONFLICT (buyer_id, client_request_id) DO NOTHING;

-- name: GetRunForBuyer :one
SELECT * FROM procurement.agent_runs WHERE id = @id AND buyer_id = @buyer_id;

-- name: InsertRunInput :exec
INSERT INTO procurement.agent_run_inputs (run_id, buyer_message, created_at) VALUES (@run_id, @buyer_message, @now);

-- name: GetRunInput :one
SELECT i.* FROM procurement.agent_run_inputs i
JOIN procurement.agent_runs r ON r.id = i.run_id
WHERE i.run_id = @run_id AND r.buyer_id = @buyer_id;

-- name: ConsumeUsage :one
-- 一列計數器加條件 UPDATE（A9-D9）：used 達上限就不更新、回 0 列（呼叫端拒絕）；沒有這一期的列時先建。
INSERT INTO procurement.usage (buyer_id, period_start, used, updated_at)
SELECT @buyer_id, @period_start, 1, @now::timestamptz WHERE @quota_limit::int > 0
ON CONFLICT (buyer_id, period_start) DO UPDATE
SET used = procurement.usage.used + 1, updated_at = EXCLUDED.updated_at
WHERE procurement.usage.used < @quota_limit::int
RETURNING used;

-- name: RefundUsage :execrows
-- 外部呼叫失敗時退回一次；used 已是 0 時不動（不會變負）。
UPDATE procurement.usage SET used = used - 1, updated_at = @now
WHERE buyer_id = @buyer_id AND period_start = @period_start AND used > 0;

-- name: GetUsage :one
SELECT used FROM procurement.usage WHERE buyer_id = @buyer_id AND period_start = @period_start;
