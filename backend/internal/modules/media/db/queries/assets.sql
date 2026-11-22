-- name: InsertAsset :exec
INSERT INTO media.assets (id, visibility, object_key, content_type, size_bytes, state, key_id, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(visibility), sqlc.arg(object_key), sqlc.arg(content_type), sqlc.arg(size_bytes),
    'STAGED', sqlc.narg(key_id), sqlc.arg(now), sqlc.arg(now));

-- name: GetAsset :one
SELECT * FROM media.assets WHERE id = sqlc.arg(id);

-- name: ActivateAsset :execrows
-- STAGED → ACTIVE（確認）。
UPDATE media.assets SET state = 'ACTIVE', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'STAGED';

-- name: MarkAssetsDeleting :many
-- 擁有者的刪除指令：只有 ACTIVE 的資產會轉 DELETING（未確認的暫存只由 GC 刪；已在刪除中或已刪的不重複處理）；不存在的 ID 忽略。
UPDATE media.assets SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND state = 'ACTIVE'
RETURNING id;

-- name: ClaimAssetsOfUploads :execrows
-- GC 階段一的另一半：被 GC 的暫存列所指的資產（還是 STAGED）一起轉 DELETING。
UPDATE media.assets SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND state = 'STAGED';

-- name: ListDeletingAssets :many
SELECT id, object_key FROM media.assets
WHERE state = 'DELETING' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: MarkAssetDeleted :execrows
UPDATE media.assets SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING';
