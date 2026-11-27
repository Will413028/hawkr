-- name: EffectiveMeasures :many
SELECT m.account_id, m.id AS measure_id, m.kind, m.termination_ground, m.basis, m.risk_ground, f.category AS lead_category
FROM seller.measures m
LEFT JOIN seller.violations f ON f.id = m.lead_finding_id
WHERE m.account_id = ANY(sqlc.arg(account_ids)::bigint[])
  AND m.id IN (
    SELECT e.id FROM seller.effective_measures(sqlc.arg(now)::timestamptz) e WHERE e.account_id = ANY(sqlc.arg(account_ids)::bigint[])
  )
ORDER BY m.account_id, m.created_at DESC, m.id DESC;

-- name: EffectiveMeasuresAll :many
SELECT m.account_id, m.id AS measure_id, m.kind, m.termination_ground, m.basis, m.risk_ground, f.category AS lead_category
FROM seller.measures m
LEFT JOIN seller.violations f ON f.id = m.lead_finding_id
WHERE m.id IN (SELECT e.id FROM seller.effective_measures(sqlc.arg(now)::timestamptz) e)
ORDER BY m.account_id, m.created_at DESC, m.id DESC;

-- name: ScheduledMeasureIDs :many
WITH timed AS (
  SELECT m.id
  FROM seller.measures m
  WHERE m.ends_at > sqlc.arg(now)::timestamptz
    AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at > sqlc.arg(now)::timestamptz
    AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id)::bigint)
), open_ended AS (
  SELECT m.id
  FROM seller.measures m
  WHERE m.ends_at IS NULL
    AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at > sqlc.arg(now)::timestamptz
    AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id)::bigint)
)
SELECT id::uuid AS id FROM (SELECT id FROM timed UNION ALL SELECT id FROM open_ended) e;

-- name: OpenSuspensionPeriods :many
SELECT p.*
FROM seller.suspension_periods p
JOIN seller.stores st ON st.store_id = p.store_id
WHERE st.account_id = $1 AND p.ended_at IS NULL
ORDER BY p.store_id;

-- name: StartSuspensionPeriod :one
INSERT INTO seller.suspension_periods (store_id, started_at)
VALUES (sqlc.arg(store_id), sqlc.arg(now))
RETURNING *;

-- name: EndSuspensionPeriod :one
UPDATE seller.suspension_periods
SET ended_at = sqlc.arg(now)
WHERE period_id = sqlc.arg(period_id) AND ended_at IS NULL
RETURNING *;


-- name: AccountsWithOpenPeriod :many
SELECT DISTINCT st.account_id
FROM seller.suspension_periods p
JOIN seller.stores st ON st.store_id = p.store_id
WHERE p.ended_at IS NULL AND st.account_id > sqlc.arg(after_account_id)
ORDER BY st.account_id
LIMIT sqlc.arg(batch_size);
