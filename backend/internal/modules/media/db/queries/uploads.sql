-- name: InsertUpload :exec
INSERT INTO media.staged_uploads (
    id, token_hash, uploader_id, uploader_admin, visibility, declared_size, declared_content_type, declared_md5,
    state, tmp_key, expires_at, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(token_hash), sqlc.arg(uploader_id), sqlc.arg(uploader_admin), sqlc.arg(visibility),
    sqlc.arg(declared_size), sqlc.arg(declared_content_type), sqlc.arg(declared_md5),
    'PENDING', sqlc.arg(tmp_key), sqlc.arg(expires_at), sqlc.arg(now), sqlc.arg(now));

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
-- GC 階段一：過期且未確認（PENDING、READY、REJECTED）的列，以及過期超過寬限仍停在 UPLOADED 的列（處理 job 已不會再來），
-- 轉 DELETING 並回傳，隨 transaction 提交；物件由 transaction 外刪。
-- 條件只在子查詢寫一次，FOR UPDATE 在等到鎖之後會重新檢查它，所以和 MarkUploadConfirmed 同時發生時只有一方成功。
UPDATE media.staged_uploads
SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE id IN (
    SELECT u.id FROM media.staged_uploads AS u
    WHERE (u.state IN ('PENDING', 'READY', 'REJECTED') AND u.expires_at <= sqlc.arg(now))
       OR (u.state = 'UPLOADED' AND u.expires_at <= sqlc.arg(uploaded_before))
    ORDER BY u.expires_at, u.id
    LIMIT sqlc.arg(max_rows)
    FOR UPDATE SKIP LOCKED)
RETURNING id, asset_id;

-- name: ListDeletingUploads :many
SELECT id, tmp_key, final_key, asset_id FROM media.staged_uploads
WHERE state = 'DELETING' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: MarkUploadDeleted :execrows
UPDATE media.staged_uploads SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING';
