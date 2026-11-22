-- name: InsertAsset :exec
INSERT INTO media.assets (id, visibility, object_key, content_type, size_bytes, state, kek_id, wrapped_dek, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(visibility), sqlc.arg(object_key), sqlc.arg(content_type), sqlc.arg(size_bytes),
    'STAGED', sqlc.narg(kek_id), sqlc.narg(wrapped_dek), sqlc.arg(now), sqlc.arg(now));

-- name: InsertSecureAssetFromUpload :exec
-- 最後一段推進後建機敏資產列：預配 ID、型別、大小、段總數、kek_id 與包好的 DEK 都從（已被這個 transaction 鎖住的）暫存列搬過來，
-- 所以搬到的一定是當時最新的 DEK 包裹；物件位置是前綴 secure/<資產 ID>/。
INSERT INTO media.assets (id, visibility, object_key, content_type, size_bytes, state, kek_id, wrapped_dek, chunk_count, created_at, updated_at)
SELECT u.planned_asset_id, 'SECURE', sqlc.arg(object_key), u.declared_content_type, u.declared_size, 'STAGED', u.kek_id, u.wrapped_dek, u.chunk_count,
    sqlc.arg(now), sqlc.arg(now)
FROM media.staged_uploads AS u
WHERE u.id = sqlc.arg(upload_id) AND u.state = 'UPLOADING' AND u.next_index = u.chunk_count;

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
SELECT id, object_key, visibility FROM media.assets
WHERE state = 'DELETING' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: MarkAssetDeleted :execrows
UPDATE media.assets SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING';

-- name: ListAssetsToRewrap :many
-- 換 KEK 後重新包裹用：keyset 分頁取 kek_id 不是 primary 的機敏資產（DELETED 除外，物件已刪、不需要再讀）。
SELECT id, kek_id, wrapped_dek FROM media.assets
WHERE visibility = 'SECURE' AND state <> 'DELETED' AND kek_id <> sqlc.arg(primary_kek_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: RewrapAsset :execrows
-- 只在列上的 kek_id 仍是取出時的舊值才更新：兩個 RewrapDEKs 同時跑、或擁有者並行讀取時只有一方生效，輸的視為已處理。
UPDATE media.assets SET kek_id = sqlc.arg(new_kek_id), wrapped_dek = sqlc.arg(new_wrapped_dek), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND kek_id = sqlc.arg(old_kek_id);
