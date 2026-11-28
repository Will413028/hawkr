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

-- name: ListPendingDeletionRequests :many
SELECT asset_id, requested_at FROM media.asset_deletion_requests
WHERE applied_at IS NULL AND (requested_at, asset_id) > (sqlc.arg(after_requested_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY requested_at, asset_id
LIMIT sqlc.arg(max_rows);

-- name: ApplyDeletionRequest :execrows
WITH marked AS (
    UPDATE media.assets SET state = 'DELETING', updated_at = sqlc.arg(now)::timestamptz
    WHERE id = sqlc.arg(asset_id)::uuid AND state = 'STAGED'
    RETURNING id)
UPDATE media.asset_deletion_requests SET applied_at = sqlc.arg(now)::timestamptz
WHERE asset_id = sqlc.arg(asset_id)::uuid AND applied_at IS NULL;

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

-- name: ListAssetStates :many
SELECT a.id, a.state, a.visibility,
    EXISTS (SELECT 1 FROM media.upload_claims AS c WHERE c.asset_id = a.id AND c.kind = 'CONFIRMED')::boolean AS confirmed
FROM media.assets AS a WHERE a.id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY a.id;

-- name: LockAssetStateShared :one
SELECT state FROM media.assets WHERE id = sqlc.arg(id)::uuid FOR SHARE;

-- name: InsertHoldGates :exec
INSERT INTO media.asset_gates (asset_id, kind, created_at)
SELECT unnest(sqlc.arg(ids)::uuid[]), 'HOLD', sqlc.arg(now)::timestamptz
ON CONFLICT (asset_id) DO NOTHING;

-- name: ListAssetGates :many
SELECT asset_id, kind FROM media.asset_gates WHERE asset_id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY asset_id;

-- name: InsertHolds :exec
INSERT INTO media.asset_holds (hold_id, asset_id, source, source_id, until, created_at)
SELECT gen_random_uuid(), t.id, sqlc.arg(source), sqlc.arg(source_id)::uuid, sqlc.arg(until)::timestamptz, sqlc.arg(now)::timestamptz
FROM unnest(sqlc.arg(ids)::uuid[]) AS t(id)
WHERE NOT EXISTS (
    SELECT 1 FROM media.asset_holds AS h
    WHERE h.asset_id = t.id AND h.source = sqlc.arg(source) AND h.source_id = sqlc.arg(source_id)::uuid
      AND h.until >= sqlc.arg(until)::timestamptz
      AND NOT EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id));

-- name: IsHoldSourceLive :one
SELECT EXISTS (
    SELECT 1 FROM media.asset_holds AS h
    WHERE h.source = sqlc.arg(source) AND h.source_id = sqlc.arg(source_id)::uuid AND h.until > sqlc.arg(now)::timestamptz
      AND NOT EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id))::boolean;

-- name: ExtendHolds :exec
INSERT INTO media.asset_holds (hold_id, asset_id, source, source_id, until, created_at)
SELECT gen_random_uuid(), l.asset_id, l.source, l.source_id, sqlc.arg(until)::timestamptz, sqlc.arg(now)::timestamptz
FROM (
    SELECT h.asset_id, h.source, h.source_id, max(h.until) AS latest
    FROM media.asset_holds AS h
    WHERE h.source = sqlc.arg(source) AND h.source_id = sqlc.arg(source_id)::uuid AND h.until > sqlc.arg(now)::timestamptz
      AND NOT EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id)
    GROUP BY h.asset_id, h.source, h.source_id) AS l
WHERE l.latest < sqlc.arg(until)::timestamptz;

-- name: ReleaseHolds :exec
INSERT INTO media.asset_hold_releases (hold_id, released_at)
SELECT h.hold_id, sqlc.arg(now)::timestamptz FROM media.asset_holds AS h
WHERE h.source = sqlc.arg(source) AND h.source_id = sqlc.arg(source_id)::uuid
  AND NOT EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id)
ON CONFLICT (hold_id) DO NOTHING;

-- name: LockHoldGate :one
SELECT asset_id FROM media.asset_gates WHERE asset_id = sqlc.arg(asset_id) AND kind = 'HOLD' FOR UPDATE;

-- name: DeleteDeadHolds :one
WITH dead AS (
    SELECT h.hold_id FROM media.asset_holds AS h
    WHERE h.asset_id = sqlc.arg(asset_id)::uuid
      AND (h.until <= sqlc.arg(now)::timestamptz OR EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id))),
gone AS (
    DELETE FROM media.asset_hold_releases WHERE hold_id IN (SELECT hold_id FROM dead)),
removed AS (
    DELETE FROM media.asset_holds WHERE hold_id IN (SELECT hold_id FROM dead) RETURNING asset_id, source, source_id)
SELECT count(*) FROM (SELECT DISTINCT asset_id, source, source_id FROM removed) AS r;

-- name: DeleteHoldsOfSource :execrows
WITH dead AS (
    SELECT h.hold_id FROM media.asset_holds AS h
    WHERE h.asset_id = sqlc.arg(asset_id)::uuid AND h.source = sqlc.arg(source) AND h.source_id = sqlc.arg(source_id)::uuid),
gone AS (
    DELETE FROM media.asset_hold_releases WHERE hold_id IN (SELECT hold_id FROM dead))
DELETE FROM media.asset_holds WHERE hold_id IN (SELECT hold_id FROM dead);

-- name: DeleteIdleHoldGate :execrows
DELETE FROM media.asset_gates AS g
WHERE g.asset_id = sqlc.arg(asset_id) AND g.kind = 'HOLD'
  AND NOT EXISTS (SELECT 1 FROM media.asset_holds AS h WHERE h.asset_id = g.asset_id);

-- name: InsertDeleteGate :exec
INSERT INTO media.asset_gates (asset_id, kind, created_at)
SELECT a.id, 'DELETE', sqlc.arg(now)::timestamptz FROM media.assets AS a WHERE a.id = sqlc.arg(asset_id)::uuid
ON CONFLICT (asset_id) DO NOTHING;

-- name: GetGateKind :one
SELECT kind FROM media.asset_gates WHERE asset_id = sqlc.arg(asset_id);

-- name: ListAssetsWithDeadHolds :many
SELECT DISTINCT h.asset_id FROM media.asset_holds AS h
WHERE h.asset_id > sqlc.arg(after_id)::uuid
  AND (h.until <= sqlc.arg(now)::timestamptz OR EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id))
ORDER BY h.asset_id
LIMIT sqlc.arg(max_rows);

-- name: ListAssetsOfHoldSource :many
SELECT DISTINCT asset_id FROM media.asset_holds WHERE source = sqlc.arg(source) AND source_id = sqlc.arg(source_id)::uuid ORDER BY asset_id;

-- name: LockDeletingAsset :one
SELECT id FROM media.assets WHERE id = sqlc.arg(id) AND state = 'DELETING' FOR NO KEY UPDATE;

-- name: IsAssetHeld :one
SELECT EXISTS (
    SELECT 1 FROM media.asset_holds AS h
    WHERE h.asset_id = sqlc.arg(asset_id) AND h.until > sqlc.arg(now)
      AND NOT EXISTS (SELECT 1 FROM media.asset_hold_releases AS r WHERE r.hold_id = h.hold_id))::boolean;

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
  AND (requested_at, asset_id) > (sqlc.arg(after_requested_at)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY requested_at, asset_id
LIMIT sqlc.arg(max_rows);

-- name: CompleteCopyRequest :execrows
UPDATE media.asset_copy_requests SET completed_at = sqlc.arg(now)
WHERE asset_id = sqlc.arg(asset_id) AND completed_at IS NULL AND failed_at IS NULL;

-- name: FailCopyRequest :execrows
UPDATE media.asset_copy_requests SET failed_at = sqlc.arg(now)
WHERE asset_id = sqlc.arg(asset_id) AND completed_at IS NULL AND failed_at IS NULL;
