-- name: ConsumeUsage :one
-- 每店一列計數器加条件 UPSERT（A9-D9）：列属于较舊的一週時重設為 1；同一週或列屬於較新的一週（pod 時鐘偏差）時只在 used 小於上限才加一，period_start 不往回移，達上限回 0 列（呼叫端拒絕）；沒有列時先建。回傳實際計數的那一週。
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
-- 外部呼叫失敗時退回一次；只動同一週的列，used 已是 0 時不動（不會變負）；回 0 列代表沒有東西可退。
UPDATE imageai.usage SET used = used - 1, updated_at = @now
WHERE store_id = @store_id AND period_start = @period_start AND used > 0;

-- name: GetUsage :one
SELECT used FROM imageai.usage WHERE store_id = @store_id AND period_start = @period_start;
