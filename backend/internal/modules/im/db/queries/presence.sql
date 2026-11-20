-- 串流的存活標記（presence 與並行上限）與 NOTIFY。時間只由 SQL 的 now() 產生與比較，沒有時間參數。

-- name: Notify :exec
SELECT pg_notify('im_events', sqlc.arg(payload)::text);

-- name: LockPartyStreams :exec
-- 同一個 Party 的訂閱以 transaction 層級的 advisory lock 序列化（不鎖業務列，不在鎖序裡）。
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(kind)::text || ':' || sqlc.arg(party_id)::bigint::text, 0));

-- name: CountLivePartyStreams :one
SELECT count(*)::bigint FROM im.presence_streams
WHERE kind = sqlc.arg(kind) AND party_id = sqlc.arg(party_id) AND expires_at > now();

-- name: InsertPresenceStream :exec
INSERT INTO im.presence_streams (stream_id, kind, party_id, expires_at)
VALUES (sqlc.arg(stream_id), sqlc.arg(kind), sqlc.arg(party_id), now() + sqlc.arg(ttl_ms)::bigint * interval '1 millisecond');

-- name: ExtendPresenceStreams :execrows
UPDATE im.presence_streams SET expires_at = now() + sqlc.arg(ttl_ms)::bigint * interval '1 millisecond'
WHERE stream_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: DeletePresenceStreams :exec
DELETE FROM im.presence_streams WHERE stream_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: IsPartyOnline :one
SELECT EXISTS (
    SELECT 1 FROM im.presence_streams
    WHERE kind = sqlc.arg(kind) AND party_id = sqlc.arg(party_id) AND expires_at > now()
);

-- name: SweepPresenceStreams :execrows
DELETE FROM im.presence_streams WHERE expires_at < now() - interval '10 minutes';
