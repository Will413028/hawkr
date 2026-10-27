-- name: LockInviteByRecipient :one
-- Ensure 鎖住那張邀請；鎖到之後同一個 RFQ 收件人的 Ensure、Mark 與撤銷都序列化。
SELECT * FROM agent.vendor_portal_invites WHERE rfq_recipient_id = sqlc.arg(rfq_recipient_id) FOR UPDATE;

-- name: InsertInviteIfAbsent :one
-- 以 rfq_recipient_id 的唯一鍵去重：併發的 Ensure 輸的一方拿到 0 列（pgx.ErrNoRows），改走鎖定既有列的分支，
-- 不會撞出 unique violation。
INSERT INTO agent.vendor_portal_invites (
    invite_id, rfq_recipient_id, prospect_id, invited_email, secret_hash, expires_at, created_at, updated_at
) VALUES (
    sqlc.arg(invite_id), sqlc.arg(rfq_recipient_id), sqlc.arg(prospect_id), sqlc.arg(invited_email), sqlc.arg(secret_hash),
    sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now)
)
ON CONFLICT (rfq_recipient_id) DO NOTHING
RETURNING invite_id;

-- name: RotateInviteSecret :execrows
-- 還沒寄出、沒撤銷的邀請換 secret、到期時間與 email 快照（條件在這裡再寫一次；呼叫方已持有列鎖，並已確認 prospect_id 相同）。
UPDATE agent.vendor_portal_invites
SET secret_hash = sqlc.arg(secret_hash),
    expires_at = sqlc.arg(expires_at),
    invited_email = sqlc.arg(invited_email),
    updated_at = sqlc.arg(now)
WHERE invite_id = sqlc.arg(invite_id) AND secret_delivered_at IS NULL AND revoked_at IS NULL;

-- name: MarkInviteDelivered :execrows
-- 單一條件 UPDATE：只有寄出的那一把 secret 仍是目前的 secret（hash 相同）才標記；已標記的不改時間。與 Ensure 的換 secret
-- 以列鎖序列化，換過之後過時的 Mark 拿到 0 列。
UPDATE agent.vendor_portal_invites
SET secret_delivered_at = sqlc.arg(now), updated_at = sqlc.arg(now)
WHERE invite_id = sqlc.arg(invite_id) AND secret_hash = sqlc.arg(secret_hash) AND secret_delivered_at IS NULL;

-- name: InviteDelivery :one
-- Mark 沒更新到列時判斷原因：邀請不存在（沒有列）、已用同一把 secret 標記（冪等）、secret 已被換掉（過時）。
SELECT secret_hash, secret_delivered_at FROM agent.vendor_portal_invites WHERE invite_id = sqlc.arg(invite_id);

-- name: LockUsableInviteBySecret :one
-- 兌換：沒撤銷、沒到期的邀請，鎖住列，讓同一張邀請的兌換、撤銷序列化（session 上限的計算才不會交錯）。
SELECT invite_id, rfq_recipient_id, expires_at
FROM agent.vendor_portal_invites
WHERE secret_hash = sqlc.arg(secret_hash) AND revoked_at IS NULL AND expires_at > sqlc.arg(now)
FOR NO KEY UPDATE;

-- name: PreviewInvite :one
SELECT invite_id, rfq_recipient_id, invited_email, expires_at
FROM agent.vendor_portal_invites
WHERE secret_hash = sqlc.arg(secret_hash) AND revoked_at IS NULL AND expires_at > sqlc.arg(now);

-- name: RevokeInvitesByProspect :many
UPDATE agent.vendor_portal_invites
SET revoked_at = sqlc.arg(now), revoke_reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE prospect_id = sqlc.arg(prospect_id) AND revoked_at IS NULL
RETURNING invite_id;

-- name: RepointInvitesToProspect :execrows
-- 合併：輸家（source）的邀請改掛贏家（target）；email 快照、secret、session 與驗證都不動，也不撤銷。rfq_recipient_id 是 UNIQUE，
-- 所以不會與贏家既有的邀請衝突。
UPDATE agent.vendor_portal_invites
SET prospect_id = sqlc.arg(target_prospect_id), updated_at = sqlc.arg(now)
WHERE prospect_id = sqlc.arg(source_prospect_id);

-- name: RevokeSessionsByProspect :execrows
-- 該 prospect 的全部邀請（含先前已撤銷的）下的有效 session 一律撤銷。
UPDATE agent.vendor_portal_sessions
SET revoked_at = sqlc.arg(now)
WHERE revoked_at IS NULL
  AND invite_id IN (SELECT invite_id FROM agent.vendor_portal_invites WHERE prospect_id = sqlc.arg(prospect_id));

-- name: PurgeInvites :execrows
-- 到期或撤銷超過保留期限的邀請（連同 email）；sessions 由 ON DELETE CASCADE 一起刪。一批最多 batch_limit 列。
DELETE FROM agent.vendor_portal_invites
WHERE invite_id IN (
    SELECT c.invite_id FROM agent.vendor_portal_invites c
    WHERE c.expires_at < sqlc.arg(cutoff) OR c.revoked_at < sqlc.arg(cutoff)
    ORDER BY c.invite_id
    LIMIT sqlc.arg(batch_limit)
);
