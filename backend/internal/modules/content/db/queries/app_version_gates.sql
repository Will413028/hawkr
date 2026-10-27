-- name: GetAppVersionGate :one
SELECT * FROM content.app_version_gates
WHERE app_type = $1;

-- name: ListAppVersionGates :many
SELECT * FROM content.app_version_gates
ORDER BY app_type;

-- name: InsertAppVersionGate :one
-- 只建立：那一組已經存在時不寫入、沒有回傳列（pgx.ErrNoRows）。
INSERT INTO content.app_version_gates (app_type, minimum_version, store_url, notice_title, notice_body)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (app_type) DO NOTHING
RETURNING *;

-- name: UpdateAppVersionGate :one
-- 條件式 UPDATE：不存在或版本不符時沒有回傳列（pgx.ErrNoRows），由呼叫方分辨。
UPDATE content.app_version_gates
SET minimum_version = $2,
    store_url       = $3,
    notice_title    = $4,
    notice_body     = $5,
    version         = version + 1,
    updated_at      = NOW()
WHERE app_type = $1 AND version = sqlc.arg(expected_version)
RETURNING *;
