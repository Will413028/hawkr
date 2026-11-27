-- name: InsertAsset :exec
INSERT INTO media.assets (id, visibility, object_key, content_type, size_bytes, state, kek_id, wrapped_dek, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(visibility), sqlc.arg(object_key), sqlc.arg(content_type), sqlc.arg(size_bytes),
    'STAGED', sqlc.narg(kek_id), sqlc.narg(wrapped_dek), sqlc.arg(now), sqlc.arg(now));

-- name: InsertSecureAssetFromUpload :exec
INSERT INTO media.assets (id, visibility, object_key, content_type, size_bytes, state, kek_id, wrapped_dek, chunk_count, created_at, updated_at)
SELECT u.planned_asset_id, 'SECURE', sqlc.arg(object_key), u.declared_content_type, u.declared_size, 'STAGED', u.kek_id, u.wrapped_dek, u.chunk_count,
    sqlc.arg(now), sqlc.arg(now)
FROM media.staged_uploads AS u
WHERE u.id = sqlc.arg(upload_id) AND u.state = 'UPLOADING' AND u.next_index = u.chunk_count;

-- name: GetAssetView :one
SELECT a.*,
    EXISTS (SELECT 1 FROM media.upload_claims AS c WHERE c.asset_id = a.id AND c.kind = 'CONFIRMED') AS confirmed,
    EXISTS (SELECT 1 FROM media.asset_deletion_requests AS r WHERE r.asset_id = a.id) AS delete_requested
FROM media.assets AS a WHERE a.id = sqlc.arg(id);

-- name: ListAssetIDs :many
SELECT id FROM media.assets ORDER BY id;

-- name: RequestDeletions :many
INSERT INTO media.asset_deletion_requests (asset_id, requested_at)
SELECT c.asset_id, sqlc.arg(now) FROM media.upload_claims AS c
WHERE c.asset_id = ANY(sqlc.arg(ids)::uuid[]) AND c.kind = 'CONFIRMED'
ORDER BY c.asset_id
ON CONFLICT (asset_id) DO NOTHING
RETURNING asset_id;

-- name: ApplyDeletionRequests :many
WITH picked AS (
    SELECT asset_id FROM media.asset_deletion_requests
    WHERE applied_at IS NULL
    ORDER BY requested_at, asset_id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED),
marked AS (
    UPDATE media.assets AS a SET state = 'DELETING', updated_at = sqlc.arg(now)::timestamptz
    FROM picked AS p WHERE a.id = p.asset_id AND a.state = 'STAGED'
    RETURNING a.id)
UPDATE media.asset_deletion_requests AS r SET applied_at = sqlc.arg(now)::timestamptz
FROM picked AS p WHERE r.asset_id = p.asset_id
RETURNING r.asset_id;

-- name: ClaimAssetsOfUploads :execrows
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
SELECT id, kek_id, wrapped_dek FROM media.assets
WHERE visibility = 'SECURE' AND state <> 'DELETED' AND kek_id <> sqlc.arg(primary_kek_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: RewrapAsset :execrows
UPDATE media.assets SET kek_id = sqlc.arg(new_kek_id), wrapped_dek = sqlc.arg(new_wrapped_dek), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND kek_id = sqlc.arg(old_kek_id);

-- name: SettleDeletedAssetUploads :execrows
UPDATE media.staged_uploads SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id IN (SELECT c.upload_id FROM media.upload_claims AS c WHERE c.asset_id = sqlc.arg(asset_id)::uuid AND c.kind = 'CONFIRMED')
  AND state IN ('READY', 'CONFIRMED');

-- name: DeleteClaimsOfAsset :execrows
DELETE FROM media.upload_claims WHERE asset_id = sqlc.arg(asset_id)::uuid;

-- name: DeleteRequestOfAsset :execrows
DELETE FROM media.asset_deletion_requests WHERE asset_id = sqlc.arg(asset_id);
