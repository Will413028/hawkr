
-- name: Notify :exec
SELECT pg_notify('im_events', sqlc.arg(payload)::text);

-- name: LockPartyStreams :exec
SELECT pg_advisory_xact_lock(hashtextextended('im.streams:' || sqlc.arg(kind)::text || ':' || sqlc.arg(party_id)::bigint::text, 0));

-- name: CountLivePartyStreams :one
SELECT count(*)::bigint FROM im.presence_streams
WHERE kind = sqlc.arg(kind) AND party_id = sqlc.arg(party_id) AND expires_at > now();

-- name: InsertPresenceStream :exec
INSERT INTO im.presence_streams (stream_id, kind, party_id, expires_at)
VALUES (sqlc.arg(stream_id), sqlc.arg(kind), sqlc.arg(party_id), now() + sqlc.arg(ttl_ms)::bigint * interval '1 millisecond');

-- name: ExtendPresenceStreams :execrows
INSERT INTO im.presence_streams (stream_id, kind, party_id, expires_at)
SELECT a.stream_id, k.kind, p.party_id, now() + sqlc.arg(ttl_ms)::bigint * interval '1 millisecond'
FROM unnest(sqlc.arg(ids)::uuid[]) WITH ORDINALITY AS a(stream_id, n)
JOIN unnest(sqlc.arg(kinds)::text[]) WITH ORDINALITY AS k(kind, n) ON k.n = a.n
JOIN unnest(sqlc.arg(party_ids)::bigint[]) WITH ORDINALITY AS p(party_id, n) ON p.n = a.n
ON CONFLICT (stream_id) DO UPDATE SET expires_at = EXCLUDED.expires_at;

-- name: DeletePresenceStreams :exec
DELETE FROM im.presence_streams WHERE stream_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: PresenceForParty :many
SELECT c.id, c.buyer_id, c.store_id,
       (CASE WHEN sqlc.arg(kind)::text = 'BUYER'
             THEN EXISTS (SELECT 1 FROM seller.pub_retired_stores() AS rs WHERE rs.store_id = c.store_id)
             ELSE EXISTS (SELECT 1 FROM buyer.pub_closed_buyers() AS cb WHERE cb.buyer_id = c.buyer_id)
        END)::boolean AS peer_inactive,
       EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() AS bs WHERE bs.blocker_buyer_id = c.buyer_id AND bs.store_id = c.store_id)::boolean AS buyer_blocks_store,
       EXISTS (SELECT 1 FROM im.presence_streams AS ps
               WHERE ps.kind = CASE WHEN sqlc.arg(kind)::text = 'BUYER' THEN 'STORE' ELSE 'BUYER' END
                 AND ps.party_id = CASE WHEN sqlc.arg(kind)::text = 'BUYER' THEN c.store_id ELSE c.buyer_id END
                 AND ps.expires_at > now())::boolean AS peer_online
FROM im.conversations AS c
WHERE c.id = ANY(sqlc.arg(ids)::uuid[])
  AND ((sqlc.arg(kind)::text = 'BUYER' AND c.buyer_id = sqlc.arg(party_id)::bigint)
    OR (sqlc.arg(kind)::text = 'STORE' AND c.store_id = sqlc.arg(party_id)::bigint));

-- name: SweepPresenceStreams :execrows
DELETE FROM im.presence_streams WHERE expires_at < now() - interval '10 minutes';
