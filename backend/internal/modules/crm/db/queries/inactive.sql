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
-- 只與 Deactivate 的獨占互斥。死結的論證：投影依序持 BUYER 一把、STORE 一把（都是共享），之後只取 crm 自己的 order_nets、customers 列鎖，
-- 不持 seller、buyer 擁有的任何鎖；等待 STORE 鎖時它只持 BUYER 共享。Deactivate 的呼叫者是買家關閉（只拿一把 BUYER 獨占）與 seller 的
-- retireBatch（同一個 transaction 依序拿多把 STORE 獨占，從不拿 BUYER），所以沒有任何 transaction 持 STORE 去等 BUYER；
-- 持有多把 STORE 獨占的 retireBatch 等的是別的投影的 STORE 共享，而那個投影拿到之後不再等任何 retireBatch 持有的鎖，會自己 commit 釋放。
SELECT pg_advisory_xact_lock_shared(hashtextextended('crm.party:' || sqlc.arg(kind)::text || ':' || sqlc.arg(party_id)::bigint::text, 0));

-- name: ListInactivePartiesWithData :many
-- 抹除的保底掃描：已在 inactive_parties 但 crm 仍有資料的身分，依 (kind, party_id) 的 keyset 分頁，最多 max_rows 個；第一頁的游標是 ('', 0)。
SELECT ip.kind, ip.party_id FROM crm.inactive_parties AS ip
WHERE (ip.kind, ip.party_id) > (sqlc.arg(after_kind)::text, sqlc.arg(after_id)::bigint)
  AND ((ip.kind = 'BUYER' AND (
          EXISTS (SELECT 1 FROM crm.customers AS c WHERE c.buyer_id = ip.party_id)
          OR EXISTS (SELECT 1 FROM crm.order_nets AS o WHERE o.buyer_id = ip.party_id)))
   OR (ip.kind = 'STORE' AND (
          EXISTS (SELECT 1 FROM crm.customers AS c WHERE c.store_id = ip.party_id)
          OR EXISTS (SELECT 1 FROM crm.order_nets AS o WHERE o.store_id = ip.party_id)
          OR EXISTS (SELECT 1 FROM crm.tags AS t WHERE t.store_id = ip.party_id))))
ORDER BY ip.kind, ip.party_id
LIMIT sqlc.arg(max_rows);
