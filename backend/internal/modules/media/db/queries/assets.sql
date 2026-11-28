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
    SELECT r.asset_id FROM media.asset_deletion_requests AS r
    WHERE r.applied_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM media.asset_holds AS h WHERE h.asset_id = r.asset_id AND h.until > sqlc.arg(now)::timestamptz)
    ORDER BY r.requested_at, r.asset_id
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
UPDATE media.assets AS a SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE a.id = ANY(sqlc.arg(ids)::uuid[]) AND a.state = 'STAGED'
  AND NOT EXISTS (SELECT 1 FROM media.asset_holds AS h WHERE h.asset_id = a.id AND h.until > sqlc.arg(now)::timestamptz);

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

-- name: LockAssetsForHold :many
SELECT id, state FROM media.assets WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY id FOR SHARE;

-- name: UpsertHolds :exec
INSERT INTO media.asset_holds (asset_id, source, source_id, until, created_at)
SELECT unnest(sqlc.arg(ids)::uuid[]), sqlc.arg(source), sqlc.arg(source_id)::uuid, sqlc.arg(until)::timestamptz, sqlc.arg(now)::timestamptz
ON CONFLICT (asset_id, source, source_id) DO UPDATE SET until = GREATEST(media.asset_holds.until, EXCLUDED.until);

-- name: ExtendHolds :execrows
UPDATE media.asset_holds SET until = GREATEST(until, sqlc.arg(until)::timestamptz)
WHERE source = sqlc.arg(source) AND source_id = sqlc.arg(source_id);

-- name: ReleaseHolds :execrows
DELETE FROM media.asset_holds WHERE source = sqlc.arg(source) AND source_id = sqlc.arg(source_id);

-- name: DeleteExpiredHolds :execrows
DELETE FROM media.asset_holds WHERE until <= sqlc.arg(now);

-- name: LockDeletingAsset :one
SELECT id FROM media.assets WHERE id = sqlc.arg(id) AND state = 'DELETING' FOR UPDATE;

-- name: IsAssetHeld :one
SELECT EXISTS (SELECT 1 FROM media.asset_holds AS h WHERE h.asset_id = sqlc.arg(asset_id) AND h.until > sqlc.arg(now))::boolean;

-- name: RestoreHeldAsset :execrows
UPDATE media.assets SET state = 'STAGED', updated_at = sqlc.arg(now) WHERE id = sqlc.arg(id) AND state = 'DELETING';

-- name: ReopenDeletionRequest :execrows
UPDATE media.asset_deletion_requests SET applied_at = NULL WHERE asset_id = sqlc.arg(asset_id);

-- name: InsertCopyAsset :exec
INSERT INTO media.assets (id, visibility, object_key, content_type, size_bytes, state, kek_id, wrapped_dek, chunk_count, created_at, updated_at)
VALUES (
    sqlc.arg(id), 'SECURE', sqlc.arg(object_key), sqlc.arg(content_type), sqlc.arg(size_bytes), 'STAGED',
    sqlc.arg(kek_id)::text, sqlc.arg(wrapped_dek), sqlc.arg(chunk_count)::integer, sqlc.arg(now), sqlc.arg(now));

-- name: InsertCopyRequest :exec
INSERT INTO media.asset_copy_requests (asset_id, source_asset_id, requested_at)
VALUES (sqlc.arg(asset_id), sqlc.arg(source_asset_id), sqlc.arg(now));

-- name: GetCopyRequest :one
SELECT * FROM media.asset_copy_requests WHERE asset_id = sqlc.arg(asset_id);

-- name: ListPendingCopyRequests :many
SELECT * FROM media.asset_copy_requests
WHERE completed_at IS NULL AND failed_at IS NULL
ORDER BY requested_at, asset_id
LIMIT sqlc.arg(max_rows);

-- name: CompleteCopyRequest :execrows
UPDATE media.asset_copy_requests SET completed_at = sqlc.arg(now)
WHERE asset_id = sqlc.arg(asset_id) AND completed_at IS NULL AND failed_at IS NULL;

-- name: FailCopyRequest :execrows
UPDATE media.asset_copy_requests SET failed_at = sqlc.arg(now)
WHERE asset_id = sqlc.arg(asset_id) AND completed_at IS NULL AND failed_at IS NULL;
