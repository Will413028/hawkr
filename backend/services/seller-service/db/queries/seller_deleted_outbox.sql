-- name: InsertSellerDeletedOutbox :one
INSERT INTO seller_deleted_outbox (id, account_id, event_type, payload, status, attempt_count, next_attempt_at, created_at)
VALUES ($1, $2, $3, $4, 'pending', 0, $5, $6) RETURNING *;

-- name: FetchPendingSellerDeletedOutbox :many
-- issue #2294 L-GO：這裡**刻意沒有** FOR UPDATE SKIP LOCKED（理由同 account-service
-- apple_revoke_outbox.sql）：呼叫端在自動提交下跑，SELECT 一回傳鎖就釋放 ⇒ 零保護。
-- 搶佔改由 ClaimSellerDeletedOutbox 的 lease CAS 負責（《多副本排程搶佔規範》§一 ②）。
SELECT * FROM seller_deleted_outbox
WHERE status = 'pending' AND next_attempt_at <= $1
ORDER BY next_attempt_at LIMIT $2;

-- name: ClaimSellerDeletedOutbox :execrows
-- issue #2294 L-GO：lease claim —— next_attempt_at 推到未來，rows=1 的副本才做副作用
--（publish 事件 ＋ account-service RemoveSellerRole gRPC）。
-- 零 schema 變更（next_attempt_at 本來就是候選述詞的時間欄），崩潰後租約到期自動回來。
UPDATE seller_deleted_outbox
SET next_attempt_at = sqlc.arg(lease_until)
WHERE id = sqlc.arg(id)
  AND status = 'pending'
  AND next_attempt_at <= sqlc.arg(claimable_before);

-- name: UpdateSellerDeletedOutboxAttempt :exec
UPDATE seller_deleted_outbox
SET attempt_count = $2, next_attempt_at = $3, last_error = $4 WHERE id = $1;

-- name: MarkSellerDeletedOutboxDone :exec
UPDATE seller_deleted_outbox SET status = 'done', completed_at = $2 WHERE id = $1;

-- name: MarkSellerDeletedOutboxFailed :exec
UPDATE seller_deleted_outbox SET status = 'failed', last_error = $2, completed_at = $3 WHERE id = $1;

-- name: MarkSellerDeletedOutboxEventPublished :exec
UPDATE seller_deleted_outbox SET event_published_at = $2 WHERE id = $1;

-- name: MarkSellerDeletedOutboxRoleRemoved :exec
UPDATE seller_deleted_outbox SET role_removed_at = $2 WHERE id = $1;

-- name: CountPendingSellerDeletedOutbox :one
SELECT COUNT(*) FROM seller_deleted_outbox WHERE status = 'pending';

-- name: OldestPendingSellerDeletedOutboxCreatedAt :one
-- Returns pgx.ErrNoRows when there are no pending rows — deliberately not a
-- nullable MIN() aggregate, so callers get an unambiguous "no rows" signal
-- instead of having to distinguish a NULL aggregate from a query error.
SELECT created_at FROM seller_deleted_outbox
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT 1;

-- name: DeleteDoneSellerDeletedOutboxBefore :execrows
-- Postgres has no DELETE ... LIMIT; the subquery bounds how many rows one
-- cleanup tick touches.
DELETE FROM seller_deleted_outbox
WHERE id IN (
  SELECT sub.id FROM seller_deleted_outbox AS sub
  WHERE sub.status = 'done' AND sub.completed_at < sqlc.arg(cutoff)
  LIMIT sqlc.arg(row_limit)
);
