-- 刪帳買家與退場店家的事實在 crm 的複本（schema.sql 的 inactive_parties）：投影與抹除 job 以它關閉「刪後重放舊事件重建資料」。

-- name: InsertInactiveParty :execrows
-- 冪等：身分不會回到營運中，所以只插入不刪除；已經有的列 0 列。
INSERT INTO crm.inactive_parties (kind, party_id, since)
VALUES (sqlc.arg(kind), sqlc.arg(party_id), sqlc.arg(since))
ON CONFLICT (kind, party_id) DO NOTHING;

-- name: AnyInactiveParty :one
-- 這筆事件的買家或店家有一方已刪帳或退場。
SELECT EXISTS (
    SELECT 1 FROM crm.inactive_parties
    WHERE (kind = 'BUYER' AND party_id = sqlc.arg(buyer_id)) OR (kind = 'STORE' AND party_id = sqlc.arg(store_id))
);

-- name: LockParty :exec
-- 「進入刪帳或退場」（Deactivate）持這把鎖的獨占模式；crm 自己的命名空間，不碰 buyer、seller 的列，不進鎖序。
SELECT pg_advisory_xact_lock(hashtextextended('crm.party:' || sqlc.arg(kind)::text || ':' || sqlc.arg(party_id)::bigint::text, 0));

-- name: LockPartyShared :exec
-- 「投影寫入」持同一把鎖的共享模式：投影之間不互相排隊（同一店家、同一買家的事件仍各自在 order_nets 與 customers 的列鎖上序列化），
-- 只與 Deactivate 的獨占互斥。投影依序持買家、店家兩把；Deactivate 一次只持一把，所以不會死結。
SELECT pg_advisory_xact_lock_shared(hashtextextended('crm.party:' || sqlc.arg(kind)::text || ':' || sqlc.arg(party_id)::bigint::text, 0));

-- name: ListInactivePartiesWithData :many
-- 抹除的保底掃描：已在 inactive_parties 但 crm 仍有資料的身分，最多 max_rows 個。
SELECT ip.kind, ip.party_id FROM crm.inactive_parties AS ip
WHERE (ip.kind = 'BUYER' AND (
          EXISTS (SELECT 1 FROM crm.customers AS c WHERE c.buyer_id = ip.party_id)
          OR EXISTS (SELECT 1 FROM crm.order_nets AS o WHERE o.buyer_id = ip.party_id)))
   OR (ip.kind = 'STORE' AND (
          EXISTS (SELECT 1 FROM crm.customers AS c WHERE c.store_id = ip.party_id)
          OR EXISTS (SELECT 1 FROM crm.order_nets AS o WHERE o.store_id = ip.party_id)
          OR EXISTS (SELECT 1 FROM crm.tags AS t WHERE t.store_id = ip.party_id)))
ORDER BY ip.kind, ip.party_id
LIMIT sqlc.arg(max_rows);
