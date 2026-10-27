-- name: InsertSession :exec
INSERT INTO agent.vendor_portal_sessions (
    session_id, token_hash, invite_id, idle_expires_at, absolute_expires_at, last_seen_at, created_at
) VALUES (
    sqlc.arg(session_id), sqlc.arg(token_hash), sqlc.arg(invite_id), sqlc.arg(idle_expires_at), sqlc.arg(absolute_expires_at),
    sqlc.arg(now), sqlc.arg(now)
);

-- name: RevokeSessionsOverCap :execrows
-- 一張邀請的有效 session 超過 keep 個時撤銷多出來的（呼叫方已鎖住那張邀請）。候選排除本次新發的 current_session_id：它在
-- 兌換當下一定未驗證，不能擠掉已驗證的既有 session；先撤最舊的未驗證，沒有未驗證的才撤最舊的已驗證。
UPDATE agent.vendor_portal_sessions
SET revoked_at = sqlc.arg(now)
WHERE session_id IN (
    SELECT c.session_id FROM agent.vendor_portal_sessions c
    WHERE c.invite_id = sqlc.arg(invite_id) AND c.revoked_at IS NULL
      AND c.idle_expires_at > sqlc.arg(now) AND c.absolute_expires_at > sqlc.arg(now)
      AND c.session_id <> sqlc.arg(current_session_id)
    ORDER BY (c.verified_at IS NOT NULL), c.created_at, c.session_id
    LIMIT GREATEST((
        SELECT count(*) FROM agent.vendor_portal_sessions a
        WHERE a.invite_id = sqlc.arg(invite_id) AND a.revoked_at IS NULL
          AND a.idle_expires_at > sqlc.arg(now) AND a.absolute_expires_at > sqlc.arg(now)
    ) - sqlc.arg(keep)::bigint, 0)
);

-- name: ValidateSession :one
-- 有效 ⇔ session 沒撤銷、idle 與絕對期限都沒到，且邀請沒撤銷、沒到期。同一個 statement 內，距上次看到已超過節流間隔（last_seen_at <= touch_before）
-- 才滑動 idle 期限（不超過絕對期限）；寫入失敗就是這個 statement 失敗，不吞錯誤。
WITH alive AS (
    SELECT s.session_id, s.invite_id, s.verification_method, s.verified_email, s.verified_at, s.last_seen_at, s.absolute_expires_at,
           i.rfq_recipient_id
    FROM agent.vendor_portal_sessions s
    JOIN agent.vendor_portal_invites i ON i.invite_id = s.invite_id
    WHERE s.token_hash = sqlc.arg(token_hash)
      AND s.revoked_at IS NULL AND s.idle_expires_at > sqlc.arg(now) AND s.absolute_expires_at > sqlc.arg(now)
      AND i.revoked_at IS NULL AND i.expires_at > sqlc.arg(now)
), touched AS (
    UPDATE agent.vendor_portal_sessions s
    SET last_seen_at = sqlc.arg(now),
        idle_expires_at = LEAST(sqlc.arg(idle_until)::timestamptz, s.absolute_expires_at)
    FROM alive a
    WHERE s.session_id = a.session_id AND a.last_seen_at <= sqlc.arg(touch_before)::timestamptz
    RETURNING s.session_id
)
SELECT session_id, invite_id, rfq_recipient_id, verification_method, verified_email, verified_at
FROM alive;

-- name: VerifySession :one
-- 驗證：有效、還沒驗證、驗證的 email（呼叫端以 platform/trimtext 去頭尾空白，不分大小寫）等於邀請的快照，單一條件 UPDATE（CAS），比對與存活條件
-- 都在述詞裡，呼叫端漏不掉。0 列（pgx.ErrNoRows）不分原因。
UPDATE agent.vendor_portal_sessions s
SET verification_method = sqlc.arg(method), verified_email = sqlc.arg(verified_email)::text, verified_at = sqlc.arg(now)
FROM agent.vendor_portal_invites i
WHERE i.invite_id = s.invite_id
  AND s.token_hash = sqlc.arg(token_hash)
  AND s.verified_at IS NULL
  AND s.revoked_at IS NULL AND s.idle_expires_at > sqlc.arg(now) AND s.absolute_expires_at > sqlc.arg(now)
  AND i.revoked_at IS NULL AND i.expires_at > sqlc.arg(now)
  AND lower(i.invited_email) = lower(sqlc.arg(verified_email)::text)
RETURNING s.session_id, s.invite_id, i.rfq_recipient_id, s.verification_method, s.verified_email, s.verified_at;

-- name: VerifiedSession :one
-- 冪等：有效、已驗證且驗證的 email 與這次相同（不分大小寫；兩邊都已去頭尾空白）的 session。
SELECT s.session_id, s.invite_id, i.rfq_recipient_id, s.verification_method, s.verified_email, s.verified_at
FROM agent.vendor_portal_sessions s
JOIN agent.vendor_portal_invites i ON i.invite_id = s.invite_id
WHERE s.token_hash = sqlc.arg(token_hash)
  AND s.verified_at IS NOT NULL AND s.verification_method = sqlc.arg(method)
  AND lower(s.verified_email) = lower(sqlc.arg(verified_email)::text)
  AND s.revoked_at IS NULL AND s.idle_expires_at > sqlc.arg(now) AND s.absolute_expires_at > sqlc.arg(now)
  AND i.revoked_at IS NULL AND i.expires_at > sqlc.arg(now);

-- name: PurgeSessions :execrows
-- 絕對期限超過保留期限的 session。一批最多 batch_limit 列。
DELETE FROM agent.vendor_portal_sessions
WHERE session_id IN (
    SELECT c.session_id FROM agent.vendor_portal_sessions c
    WHERE c.absolute_expires_at < sqlc.arg(cutoff)
    ORDER BY c.session_id
    LIMIT sqlc.arg(batch_limit)
);
