-- name: InsertUpload :exec
INSERT INTO media.staged_uploads (
    id, token_hash, uploader_id, uploader_admin, visibility, declared_size, declared_content_type, declared_md5,
    state, tmp_key, expires_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(token_hash), sqlc.arg(uploader_id), sqlc.arg(uploader_admin), sqlc.arg(visibility),
    sqlc.arg(declared_size), sqlc.arg(declared_content_type)::text, sqlc.arg(declared_md5),
    'PENDING', sqlc.arg(tmp_key), sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

-- name: InsertSecureUpload :exec
INSERT INTO media.staged_uploads (
    id, token_hash, uploader_id, uploader_admin, delegated_store_id, visibility, declared_size,
    state, planned_asset_id, chunk_count, next_index, kek_id, wrapped_dek, expires_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(token_hash), sqlc.arg(uploader_id), sqlc.arg(uploader_admin), sqlc.narg(delegated_store_id)::bigint, 'SECURE', sqlc.arg(declared_size),
    'UPLOADING', sqlc.arg(planned_asset_id)::uuid, sqlc.arg(chunk_count)::integer, 0, sqlc.arg(kek_id)::text, sqlc.arg(wrapped_dek),
    sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

-- name: AdvanceSecureUpload :execrows
UPDATE media.staged_uploads
SET next_index = next_index + 1, declared_content_type = COALESCE(declared_content_type, sqlc.narg(content_type)::text), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND next_index = sqlc.arg(chunk_index)::integer AND expires_at > sqlc.arg(now);

-- name: RejectSecureUploadAtFirstChunk :execrows
UPDATE media.staged_uploads
SET state = 'REJECTED', reject_reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND next_index = 0;

-- name: MarkSecureUploadReady :execrows
UPDATE media.staged_uploads
SET state = 'READY', asset_id = planned_asset_id, final_key = sqlc.arg(final_key), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND next_index = chunk_count;

-- name: InsertSecureChunk :exec
INSERT INTO media.secure_chunks (asset_id, idx, object_key)
VALUES (sqlc.arg(asset_id), sqlc.arg(idx), sqlc.arg(object_key));

-- name: GetSecureChunk :one
SELECT * FROM media.secure_chunks WHERE asset_id = sqlc.arg(asset_id) AND idx = sqlc.arg(idx);

-- name: DeleteSecureChunks :exec
DELETE FROM media.secure_chunks WHERE asset_id = sqlc.arg(asset_id);

-- name: ListSecureChunks :many
SELECT idx, object_key FROM media.secure_chunks WHERE asset_id = sqlc.arg(asset_id) ORDER BY idx;

-- name: ListUploadsToRewrap :many
SELECT id, planned_asset_id, kek_id, wrapped_dek FROM media.staged_uploads
WHERE state = 'UPLOADING' AND kek_id <> sqlc.arg(primary_kek_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: RewrapUpload :execrows
UPDATE media.staged_uploads SET kek_id = sqlc.arg(new_kek_id), wrapped_dek = sqlc.arg(new_wrapped_dek), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND kek_id = sqlc.arg(old_kek_id);

-- name: GetUploadByTokenHash :one
SELECT * FROM media.staged_uploads WHERE token_hash = sqlc.arg(token_hash);

-- name: GetUpload :one
SELECT * FROM media.staged_uploads WHERE id = sqlc.arg(id);

-- name: MarkUploadUploaded :execrows
UPDATE media.staged_uploads
SET state = 'UPLOADED', final_key = sqlc.arg(final_key), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'PENDING' AND expires_at > sqlc.arg(now);

-- name: MarkUploadReady :execrows
UPDATE media.staged_uploads
SET state = 'READY', asset_id = sqlc.arg(asset_id), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADED' AND expires_at > sqlc.arg(now);

-- name: MarkUploadRejected :execrows
UPDATE media.staged_uploads
SET state = 'REJECTED', reject_reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADED';

-- name: ClaimUpload :one
INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at)
SELECT u.id, 'CONFIRMED', u.asset_id, sqlc.arg(now)
FROM media.staged_uploads AS u
WHERE u.id = sqlc.arg(id) AND u.state = 'READY' AND u.expires_at > sqlc.arg(now)
ON CONFLICT DO NOTHING
RETURNING asset_id;

-- name: GetClaim :one
SELECT upload_id, kind, asset_id, (claimed_xid = pg_current_xact_id())::boolean AS here
FROM media.upload_claims WHERE upload_id = sqlc.arg(upload_id);

-- name: GetUploadView :one
SELECT u.*, EXISTS (
    SELECT 1 FROM media.upload_claims AS c WHERE c.upload_id = u.id AND c.kind = 'CONFIRMED') AS confirmed
FROM media.staged_uploads AS u WHERE u.id = sqlc.arg(id);

-- name: ListReclaimableUploads :many
SELECT u.id, u.expires_at FROM media.staged_uploads AS u
WHERE ((u.state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED') AND u.expires_at <= sqlc.arg(now))
    OR (u.state = 'UPLOADED' AND u.expires_at <= sqlc.arg(uploaded_before)))
  AND (u.expires_at, u.id) > (sqlc.arg(after_expires_at)::timestamptz, sqlc.arg(after_id)::uuid)
  AND NOT EXISTS (SELECT 1 FROM media.upload_claims AS c WHERE c.upload_id = u.id)
ORDER BY u.expires_at, u.id
LIMIT sqlc.arg(max_rows);

-- name: ReclaimUpload :one
INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at)
SELECT u.id, 'RECLAIMED', u.asset_id, sqlc.arg(now)
FROM media.staged_uploads AS u
WHERE u.id = sqlc.arg(id)
  AND ((u.state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED') AND u.expires_at <= sqlc.arg(now))
    OR (u.state = 'UPLOADED' AND u.expires_at <= sqlc.arg(uploaded_before)))
ON CONFLICT DO NOTHING
RETURNING upload_id;

-- name: MarkReclaimedUploadDeleting :one
UPDATE media.staged_uploads
SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state IN ('PENDING', 'UPLOADING', 'UPLOADED', 'READY', 'REJECTED')
RETURNING asset_id, planned_asset_id;

-- name: ListDeletingUploads :many
SELECT id, tmp_key, final_key, asset_id, planned_asset_id FROM media.staged_uploads
WHERE state = 'DELETING' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: MarkUploadDeleted :execrows
UPDATE media.staged_uploads SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING';

-- name: DeleteClaimOfUpload :execrows
DELETE FROM media.upload_claims WHERE upload_id = sqlc.arg(upload_id) AND kind = 'RECLAIMED';

-- name: SettleConfirmedUploads :execrows
UPDATE media.staged_uploads AS u SET state = 'CONFIRMED', updated_at = sqlc.arg(now)
WHERE u.id IN (
    SELECT u2.id FROM media.staged_uploads AS u2
    JOIN media.upload_claims AS c ON c.upload_id = u2.id AND c.kind = 'CONFIRMED'
    WHERE u2.state = 'READY'
    ORDER BY u2.id
    LIMIT sqlc.arg(max_rows)
    FOR NO KEY UPDATE OF u2 SKIP LOCKED)
  AND u.state = 'READY';

-- name: LockDeletingUpload :one
SELECT id, asset_id, planned_asset_id FROM media.staged_uploads WHERE id = sqlc.arg(id) AND state = 'DELETING' FOR NO KEY UPDATE;

-- name: RestoreHeldUpload :execrows
UPDATE media.staged_uploads SET state = 'READY', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING' AND asset_id IS NOT NULL AND reject_reason IS NULL;

-- name: InsertCopyUpload :exec
INSERT INTO media.staged_uploads (
    id, token_hash, uploader_id, uploader_admin, delegated_store_id, visibility, declared_size, declared_content_type,
    state, final_key, asset_id, planned_asset_id, chunk_count, next_index, kek_id, wrapped_dek, expires_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(token_hash), sqlc.arg(uploader_id), sqlc.arg(uploader_admin), sqlc.narg(delegated_store_id)::bigint, 'SECURE',
    sqlc.arg(declared_size), sqlc.arg(declared_content_type)::text, 'CONFIRMED', sqlc.arg(final_key), sqlc.arg(asset_id)::uuid, sqlc.arg(asset_id)::uuid,
    sqlc.arg(chunk_count)::integer, sqlc.arg(chunk_count)::integer, sqlc.arg(kek_id)::text, sqlc.arg(wrapped_dek), sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

-- name: InsertConfirmedClaim :exec
INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at)
VALUES (sqlc.arg(upload_id), 'CONFIRMED', sqlc.arg(asset_id), sqlc.arg(now));

-- name: InsertOrphanObject :exec
INSERT INTO media.orphan_objects (object_key, recorded_at)
VALUES (sqlc.arg(object_key), sqlc.arg(now))
ON CONFLICT (object_key) DO NOTHING;

-- name: ListOrphanObjects :many
SELECT object_key FROM media.orphan_objects
WHERE object_key > sqlc.arg(after_key)
ORDER BY object_key
LIMIT sqlc.arg(max_rows);

-- name: IsChunkObjectReferenced :one
SELECT EXISTS (SELECT 1 FROM media.secure_chunks WHERE object_key = sqlc.arg(object_key))::boolean;

-- name: DeleteOrphanObject :exec
DELETE FROM media.orphan_objects WHERE object_key = sqlc.arg(object_key);
