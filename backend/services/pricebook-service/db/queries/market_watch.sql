-- name: LockMarketWatchAccount :exec
SELECT pg_advisory_xact_lock(sqlc.arg(account_id)::bigint);

-- name: FindMarketWatchTarget :one
SELECT w.id
FROM market_watch_target w
JOIN crop c ON c.id = w.crop_id
LEFT JOIN market m ON m.id = w.market_id
WHERE w.account_id = sqlc.arg(account_id)
  AND c.crop_code = sqlc.arg(crop_code)
  AND ((sqlc.arg(market_code)::text = '' AND w.market_id IS NULL)
       OR m.market_code = sqlc.arg(market_code)::text);

-- name: ResolveMarketWatchReferences :one
SELECT c.id AS crop_id, m.id AS market_id
FROM crop c
LEFT JOIN market m ON m.market_code = sqlc.arg(market_code)::text AND m.active
WHERE c.crop_code = sqlc.arg(crop_code) AND c.active
  AND (sqlc.arg(market_code)::text = '' OR m.id IS NOT NULL);

-- name: CountMarketWatchTargets :one
SELECT count(*) FROM market_watch_target WHERE account_id = $1;

-- name: InsertMarketWatchTarget :exec
INSERT INTO market_watch_target (id, account_id, crop_id, market_id)
VALUES ($1, $2, $3, $4);

-- name: DeleteMarketWatchTarget :exec
DELETE FROM market_watch_target WHERE account_id = $1 AND id = $2;

-- name: DeleteMarketWatchTargetsByAccountID :exec
DELETE FROM market_watch_target WHERE account_id = $1;
