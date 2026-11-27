-- name: ConsumeUsage :one
INSERT INTO imageai.usage (store_id, period_start, used, updated_at)
SELECT @store_id, @period_start, 1, @now::timestamptz WHERE @quota_limit::int > 0
ON CONFLICT (store_id) DO UPDATE
SET period_start = GREATEST(imageai.usage.period_start, EXCLUDED.period_start),
    used = CASE WHEN imageai.usage.period_start < EXCLUDED.period_start THEN 1 ELSE imageai.usage.used + 1 END,
    updated_at = EXCLUDED.updated_at
WHERE imageai.usage.period_start < EXCLUDED.period_start
   OR imageai.usage.used < @quota_limit::int
RETURNING used, period_start;

-- name: RefundUsage :execrows
UPDATE imageai.usage SET used = used - 1, updated_at = @now
WHERE store_id = @store_id AND period_start = @period_start AND used > 0;

-- name: GetUsage :one
SELECT used FROM imageai.usage WHERE store_id = @store_id AND period_start = @period_start;
