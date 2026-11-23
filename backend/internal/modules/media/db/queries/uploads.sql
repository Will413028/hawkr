-- name: InsertUpload :exec
INSERT INTO media.staged_uploads (
    id, token_hash, uploader_id, uploader_admin, visibility, declared_size, declared_content_type, declared_md5,
    state, tmp_key, expires_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(token_hash), sqlc.arg(uploader_id), sqlc.arg(uploader_admin), sqlc.arg(visibility),
    sqlc.arg(declared_size), sqlc.arg(declared_content_type)::text, sqlc.arg(declared_md5),
    'PENDING', sqlc.arg(tmp_key), sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

-- name: InsertSecureUpload :exec
-- 機敏分段上傳：以 UPLOADING 建列（沒有預簽，tmp_key、declared_md5、declared_content_type 與 final_key 是 NULL），
-- 預配資產 ID、段總數、next_index 0 與包好的 DEK。
INSERT INTO media.staged_uploads (
    id, token_hash, uploader_id, uploader_admin, delegated_store_id, visibility, declared_size,
    state, planned_asset_id, chunk_count, next_index, kek_id, wrapped_dek, expires_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(token_hash), sqlc.arg(uploader_id), sqlc.arg(uploader_admin), sqlc.narg(delegated_store_id)::bigint, 'SECURE', sqlc.arg(declared_size),
    'UPLOADING', sqlc.arg(planned_asset_id)::uuid, sqlc.arg(chunk_count)::integer, 0, sqlc.arg(kek_id)::text, sqlc.arg(wrapped_dek),
    sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

-- name: AdvanceSecureUpload :execrows
-- 推進一段：只有 next_index 等於這段的序號、仍在 UPLOADING 且未過期的會話才推進（輸給並行的同一段、過期或已被拒就 0 列）。
-- 第 0 段同時記下辨認出的型別。
UPDATE media.staged_uploads
SET next_index = next_index + 1, declared_content_type = COALESCE(declared_content_type, sqlc.narg(content_type)::text), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND next_index = sqlc.arg(chunk_index)::integer AND expires_at > sqlc.arg(now);

-- name: RejectSecureUploadAtFirstChunk :execrows
-- 第 0 段辨認不出：只在還沒收任何段（next_index = 0）的 UPLOADING 會話轉 REJECTED。
UPDATE media.staged_uploads
SET state = 'REJECTED', reject_reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND next_index = 0;

-- name: MarkSecureUploadReady :execrows
-- 最後一段推進後（同一個 transaction，資產列已建）：UPLOADING → READY，記資產與前綴。
UPDATE media.staged_uploads
SET state = 'READY', asset_id = planned_asset_id, final_key = sqlc.arg(final_key), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND next_index = chunk_count;

-- name: InsertSecureChunk :exec
INSERT INTO media.secure_chunks (asset_id, idx, object_key)
VALUES (sqlc.arg(asset_id), sqlc.arg(idx), sqlc.arg(object_key));

-- name: GetSecureChunk :one
SELECT * FROM media.secure_chunks WHERE asset_id = sqlc.arg(asset_id) AND idx = sqlc.arg(idx);

-- name: DeleteSecureChunks :exec
-- 回收：資產（或暫存列預配的資產）標為 DELETED 的同一個 transaction 刪掉它的段記錄；物件已在 transaction 外刪完。
DELETE FROM media.secure_chunks WHERE asset_id = sqlc.arg(asset_id);

-- name: ListSecureChunks :many
SELECT idx, object_key FROM media.secure_chunks WHERE asset_id = sqlc.arg(asset_id) ORDER BY idx;

-- name: ListUploadsToRewrap :many
-- 換 KEK 後重新包裹用：還在上傳中、kek_id 不是 primary 的機敏暫存列（資產列建立前，DEK 只在這裡）。
SELECT id, planned_asset_id, kek_id, wrapped_dek FROM media.staged_uploads
WHERE state = 'UPLOADING' AND kek_id <> sqlc.arg(primary_kek_id) AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: RewrapUpload :execrows
-- 與 RewrapAsset 同理：kek_id 仍是取出時的舊值且還在 UPLOADING 才更新；最後一段的推進會鎖同一列，所以搬到資產列的一定是當時最新的值。
UPDATE media.staged_uploads SET kek_id = sqlc.arg(new_kek_id), wrapped_dek = sqlc.arg(new_wrapped_dek), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADING' AND kek_id = sqlc.arg(old_kek_id);

-- name: GetUploadByTokenHash :one
SELECT * FROM media.staged_uploads WHERE token_hash = sqlc.arg(token_hash);

-- name: GetUpload :one
SELECT * FROM media.staged_uploads WHERE id = sqlc.arg(id);

-- name: MarkUploadUploaded :execrows
-- PENDING → UPLOADED；列已過期就不轉。
UPDATE media.staged_uploads
SET state = 'UPLOADED', final_key = sqlc.arg(final_key), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'PENDING' AND expires_at > sqlc.arg(now);

-- name: MarkUploadReady :execrows
-- UPLOADED → READY；列已過期就不轉（處理 job 隨後刪掉剛寫的正式物件）。
UPDATE media.staged_uploads
SET state = 'READY', asset_id = sqlc.arg(asset_id), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADED' AND expires_at > sqlc.arg(now);

-- name: MarkUploadRejected :execrows
-- UPLOADED → REJECTED，記原因代碼。
UPDATE media.staged_uploads
SET state = 'REJECTED', reject_reason = sqlc.arg(reason), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'UPLOADED';

-- name: MarkUploadConfirmed :one
-- READY → CONFIRMED；只接受未過期的列，回傳該列的 asset_id（READY 一定有）。與 GC 的 ClaimExpiredUploads 搶同一列，輸的一方沒有列（pgx.ErrNoRows）。
UPDATE media.staged_uploads
SET state = 'CONFIRMED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'READY' AND expires_at > sqlc.arg(now)
RETURNING asset_id;

-- name: ClaimExpiredUploads :many
-- GC 階段一：過期且未確認（PENDING、UPLOADING、READY、REJECTED）的列，以及過期超過寬限仍停在 UPLOADED 的列（處理 job 已不會再來），
-- 轉 DELETING 並回傳，隨 transaction 提交；物件由 transaction 外刪。
-- 條件只在子查詢寫一次，FOR UPDATE 在等到鎖之後會重新檢查它，所以和 MarkUploadConfirmed 同時發生時只有一方成功。
UPDATE media.staged_uploads
SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE id IN (
    SELECT u.id FROM media.staged_uploads AS u
    WHERE (u.state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED') AND u.expires_at <= sqlc.arg(now))
       OR (u.state = 'UPLOADED' AND u.expires_at <= sqlc.arg(uploaded_before))
    ORDER BY u.expires_at, u.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
RETURNING id, asset_id;

-- name: ListDeletingUploads :many
SELECT id, tmp_key, final_key, asset_id, planned_asset_id FROM media.staged_uploads
WHERE state = 'DELETING' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: MarkUploadDeleted :execrows
UPDATE media.staged_uploads SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING';

-- name: UploadConfirmedInThisTx :one
-- 這筆暫存列是不是被目前的 transaction 轉成 CONFIRMED 的（列的 xmin 是最後寫入它的 transaction）；Confirm 重入只在同一個 transaction 內冪等（B1-D23）。
SELECT (xmin = pg_current_xact_id()::xid)::boolean AS confirmed_here FROM media.staged_uploads WHERE id = sqlc.arg(id);
