-- name: ConsumeUsage :one
-- 一列計數器加條件 UPDATE（A9-D9）：used 達上限就不更新、回 0 列（呼叫端拒絕）；沒有這一期的列時先建。
INSERT INTO imageai.usage (store_id, period_start, used, updated_at)
SELECT @store_id, @period_start, 1, @now::timestamptz WHERE @quota_limit::int > 0
ON CONFLICT (store_id, period_start) DO UPDATE
SET used = imageai.usage.used + 1, updated_at = EXCLUDED.updated_at
WHERE imageai.usage.used < @quota_limit::int
RETURNING used;

-- name: RefundUsage :execrows
-- 外部呼叫失敗時退回一次；used 已是 0 時不動（不會變負）。
UPDATE imageai.usage SET used = used - 1, updated_at = @now
WHERE store_id = @store_id AND period_start = @period_start AND used > 0;

-- name: GetUsage :one
SELECT used FROM imageai.usage WHERE store_id = @store_id AND period_start = @period_start;
