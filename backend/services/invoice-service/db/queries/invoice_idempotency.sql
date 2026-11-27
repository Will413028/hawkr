-- name: ClaimIdempotencyKey :one
WITH ins AS (
  INSERT INTO invoice_idempotency_keys (scope, actor_id, idem_key, request_hash, created_at)
  VALUES (sqlc.arg(scope), sqlc.arg(actor_id), sqlc.arg(idem_key), sqlc.arg(request_hash), sqlc.arg(now))
  ON CONFLICT (scope, actor_id, idem_key) DO NOTHING
  RETURNING scope, actor_id, idem_key, request_hash, result_ref, created_at
)
SELECT scope, actor_id, idem_key, request_hash, result_ref, created_at, true AS claimed FROM ins
UNION ALL
SELECT scope, actor_id, idem_key, request_hash, result_ref, created_at, false AS claimed
  FROM invoice_idempotency_keys
 WHERE scope = sqlc.arg(scope) AND actor_id = sqlc.arg(actor_id) AND idem_key = sqlc.arg(idem_key)
   AND NOT EXISTS (SELECT 1 FROM ins)
LIMIT 1;

-- name: SetIdempotencyResult :execrows
UPDATE invoice_idempotency_keys
   SET result_ref = sqlc.arg(result_ref)
 WHERE scope = sqlc.arg(scope) AND actor_id = sqlc.arg(actor_id) AND idem_key = sqlc.arg(idem_key);
