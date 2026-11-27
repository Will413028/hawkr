-- name: GetAppVersionGate :one
SELECT * FROM content.app_version_gates
WHERE app_type = $1;

-- name: ListAppVersionGates :many
SELECT * FROM content.app_version_gates
ORDER BY app_type;

-- name: InsertAppVersionGate :one
INSERT INTO content.app_version_gates (app_type, minimum_version, store_url, notice_title, notice_body)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (app_type) DO NOTHING
RETURNING *;

-- name: UpdateAppVersionGate :one
UPDATE content.app_version_gates
SET minimum_version = $2,
    store_url       = $3,
    notice_title    = $4,
    notice_body     = $5,
    version         = version + 1,
    updated_at      = NOW()
WHERE app_type = $1 AND version = sqlc.arg(expected_version)
RETURNING *;
