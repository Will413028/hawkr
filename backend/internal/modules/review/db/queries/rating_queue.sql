
-- name: RequestRatingRecompute :exec
INSERT INTO review.rating_recompute_queue AS q (product_id, due_at, priority)
SELECT p, sqlc.arg(now)::timestamptz, 0 FROM (SELECT DISTINCT unnest(sqlc.arg(product_ids)::uuid[]) AS p ORDER BY p) AS ids
ON CONFLICT (product_id) DO UPDATE SET
    seq = q.seq + 1,
    due_at = CASE WHEN q.failed_at IS NOT NULL THEN EXCLUDED.due_at ELSE LEAST(q.due_at, EXCLUDED.due_at) END,
    attempts = CASE WHEN q.failed_at IS NOT NULL THEN 0 ELSE q.attempts END,
    last_error = CASE WHEN q.failed_at IS NOT NULL THEN NULL ELSE q.last_error END,
    failed_at = NULL,
    priority = 0;

-- name: RequestRatingReconcileBatch :many
WITH picked AS (
    SELECT DISTINCT r.product_id FROM review.reviews AS r
    WHERE r.product_id > sqlc.arg(after_id)::uuid
    ORDER BY r.product_id
    LIMIT sqlc.arg(max_rows)),
ins AS (
    INSERT INTO review.rating_recompute_queue AS q (product_id, due_at, priority)
    SELECT p.product_id, sqlc.arg(now)::timestamptz, 1 FROM picked AS p ORDER BY p.product_id
    ON CONFLICT (product_id) DO UPDATE SET
        seq = q.seq + 1,
        due_at = LEAST(q.due_at, EXCLUDED.due_at),
        priority = LEAST(q.priority, EXCLUDED.priority)
    WHERE q.failed_at IS NULL)
SELECT p.product_id FROM picked AS p ORDER BY p.product_id;

-- name: ClaimRatingRecompute :many
WITH picked AS (
    SELECT product_id FROM review.rating_recompute_queue
    WHERE failed_at IS NULL AND due_at <= sqlc.arg(now)::timestamptz AND (claimed_until IS NULL OR claimed_until < sqlc.arg(now)::timestamptz)
    ORDER BY priority, due_at, product_id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
UPDATE review.rating_recompute_queue AS q SET claimed_until = sqlc.arg(lease_until)::timestamptz, claim_token = sqlc.arg(claim_token)::uuid
FROM picked AS p WHERE q.product_id = p.product_id
RETURNING q.product_id, q.seq, q.attempts;

-- name: FinishRatingRecompute :execrows
DELETE FROM review.rating_recompute_queue WHERE product_id = sqlc.arg(product_id) AND seq = sqlc.arg(seq) AND claim_token = sqlc.arg(claim_token)::uuid;

-- name: RearmRatingRecompute :execrows
UPDATE review.rating_recompute_queue SET claimed_until = NULL, claim_token = NULL, due_at = sqlc.arg(due_at)
WHERE product_id = sqlc.arg(product_id) AND claim_token = sqlc.arg(claim_token)::uuid;

-- name: FailRatingRecompute :execrows
UPDATE review.rating_recompute_queue SET attempts = attempts + 1, due_at = sqlc.arg(due_at), claimed_until = NULL, claim_token = NULL, last_error = sqlc.arg(last_error), failed_at = sqlc.narg(failed_at)
WHERE product_id = sqlc.arg(product_id) AND seq = sqlc.arg(seq) AND claim_token = sqlc.arg(claim_token)::uuid;
