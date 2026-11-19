-- 買家刪帳的事實在 review 的複本（schema.sql 的 inactive_authors）：公開可讀的述詞引用它。

-- name: InsertInactiveAuthor :execrows
-- 冪等：買家的狀態不會回到營運中，所以只插入不刪除；已經有的列 0 列。
INSERT INTO review.inactive_authors (buyer_id, since)
VALUES (sqlc.arg(buyer_id), sqlc.arg(since))
ON CONFLICT (buyer_id) DO NOTHING;

-- name: IsInactiveAuthor :one
SELECT EXISTS (SELECT 1 FROM review.inactive_authors WHERE buyer_id = sqlc.arg(buyer_id));
