-- 買家刪帳的事實在 review 的複本（schema.sql 的 inactive_authors）：公開可讀的述詞引用它。

-- name: InsertInactiveAuthor :execrows
-- 冪等：買家的狀態不會回到營運中，所以只插入不刪除；已經有的列 0 列。
INSERT INTO review.inactive_authors (buyer_id, since)
VALUES (sqlc.arg(buyer_id), sqlc.arg(since))
ON CONFLICT (buyer_id) DO NOTHING;

-- name: IsInactiveAuthor :one
SELECT EXISTS (SELECT 1 FROM review.inactive_authors WHERE buyer_id = sqlc.arg(buyer_id));

-- name: LockAuthor :exec
-- 同一位作者的「進入刪帳」與「提交評價」以 transaction 級 advisory lock 串行（Submit 沒有可鎖的列：評價列還不存在）；
-- 鎖的是 review 自己的命名空間，不碰 identity、buyer、seller 的列，不進鎖序。
SELECT pg_advisory_xact_lock(hashtextextended('review.author:' || sqlc.arg(buyer_id)::bigint::text, 0));
