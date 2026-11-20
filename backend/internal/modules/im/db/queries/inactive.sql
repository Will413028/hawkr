-- 刪帳者與退場店家的事實在 im 的複本（schema.sql 的 inactive_parties）：送出以它擋寄件人與收件人。

-- name: InsertInactiveParty :execrows
-- 冪等：身分不會回到營運中，所以只插入不刪除；已經有的列 0 列。
INSERT INTO im.inactive_parties (kind, party_id, since)
VALUES (sqlc.arg(kind), sqlc.arg(party_id), sqlc.arg(since))
ON CONFLICT (kind, party_id) DO NOTHING;

-- name: IsInactiveParty :one
SELECT EXISTS (SELECT 1 FROM im.inactive_parties WHERE kind = sqlc.arg(kind) AND party_id = sqlc.arg(party_id));

-- name: LockParty :exec
-- 同一個身分的「進入刪帳或退場」與「送出」以 transaction 級 advisory lock 串行；鎖的是 im 自己的命名空間，
-- 不碰 identity、buyer、seller 的列，不進鎖序。
SELECT pg_advisory_xact_lock(hashtextextended('im.party:' || sqlc.arg(kind)::text || ':' || sqlc.arg(party_id)::bigint::text, 0));
