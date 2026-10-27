-- name: ActiveMembership :one
-- 讀取 stint 的 active 指派。
SELECT *
FROM buyer.memberships
WHERE buyer_id = $1 AND status = 'ACTIVE';

-- name: MembershipRequest :one
-- 查詢request_id 的既有指派。
SELECT *
FROM buyer.memberships
WHERE request_id = $1;

-- name: LockMembershipRequest :exec
-- 序列化同 request_id 的跨買家指派。
SELECT pg_advisory_xact_lock(hashtextextended(@request_id::text, 4817));

-- name: Supersede :exec
-- 使舊的 active 指派退為歷史。
UPDATE buyer.memberships
SET status = 'SUPERSEDED'
WHERE buyer_id = $1 AND status = 'ACTIVE';

-- name: AssignMembership :one
-- 建立一筆有期間與冪等鍵的會員指派。
INSERT INTO buyer.memberships (id, buyer_id, plan_code, period_start, period_end, source, status, request_id, granted_by, note, created_at)
VALUES ($1, $2, $3, $4, $5, 'ADMIN', 'ACTIVE', $6, $7, $8, $4)
RETURNING *;
