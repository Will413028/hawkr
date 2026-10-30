-- name: GetIdentity :one
SELECT * FROM identity.account_identities WHERE provider = $1 AND subject = $2;

-- name: InsertIdentity :one
-- 同一個 (provider, subject) 並發連結時只有一個成功：沒有列回傳表示它已存在，呼叫方放棄這個 transaction 重來。
INSERT INTO identity.account_identities (provider, subject, account_id, email, email_verified, auto_link_eligible, created_at, updated_at)
VALUES (sqlc.arg(provider), sqlc.arg(subject), sqlc.arg(account_id), sqlc.narg(email), sqlc.arg(email_verified), sqlc.arg(auto_link_eligible), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (provider, subject) DO NOTHING
RETURNING account_id;

-- name: UpdateIdentityClaims :exec
-- 每次登入以 provider 最近回報的 email 更新。
UPDATE identity.account_identities
SET email = sqlc.narg(email), email_verified = sqlc.arg(email_verified), auto_link_eligible = sqlc.arg(auto_link_eligible), updated_at = sqlc.arg(now)::timestamptz
WHERE provider = sqlc.arg(provider) AND subject = sqlc.arg(subject);

-- name: ScrubIdentitiesOfAccount :exec
-- 帳號刪除時身分列留作墓碑（保留 provider 與 subject，同一個登入身分不會再建一個帳號），清掉 email。
UPDATE identity.account_identities
SET email = NULL, email_verified = FALSE, auto_link_eligible = FALSE, updated_at = sqlc.arg(now)::timestamptz
WHERE account_id = sqlc.arg(account_id);

-- name: FindAutoLinkAccountIDsByEmail :many
-- 自動連結的候選：存活帳號上符合自動連結資格的 Google 身分（email 已驗證、gmail.com 或帶 hd）裡，email 等於 email 的帳號，
-- 最多回 2 筆，讓呼叫方分辨「唯一」與「不唯一」。墓碑身分的 email 已清、帳號也已刪，不會被比對到；accounts.email 不參與。
SELECT DISTINCT i.account_id FROM identity.account_identities i
JOIN identity.accounts a ON a.id = i.account_id
WHERE a.deleted_at IS NULL AND i.provider = 'GOOGLE' AND i.email_verified AND i.auto_link_eligible
  AND lower(i.email) = lower(sqlc.arg(email)::text)
ORDER BY i.account_id
LIMIT 2;

-- name: ListIdentitiesOfAccount :many
-- 帳號已連結的登入身分，只取 provider 與連結時間（不取 subject、email）。同一 provider 可有多列（schema 沒有 (account_id, provider) 唯一約束）。
-- 墓碑身分屬於已刪除的帳號，呼叫者是存活帳號，不會列到；account_id 條件是唯一的範圍限制。
SELECT provider, created_at FROM identity.account_identities
WHERE account_id = $1
ORDER BY created_at, provider, subject;
