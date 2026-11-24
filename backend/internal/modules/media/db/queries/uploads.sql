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

-- name: ClaimUpload :one
-- 擁有者確認（B1-D25）：READY 且未過期的暫存列認領為 CONFIRMED，只新增一列、不更新任何既有列。與 GC 的 ReclaimUpload 搶同一個唯一鍵 upload_id，
-- 輸的一方沒有列（pgx.ErrNoRows）；ON CONFLICT 不指定目標：upload_id 與 asset_id 兩個唯一鍵都要吸收，並行確認同一個 token 才不會撞 upload_claims_asset_key 而回 23505；同一個鍵被另一個還沒提交的 transaction 插入時會等它結束。回傳認領的資產 ID。
INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at)
SELECT u.id, 'CONFIRMED', u.asset_id, sqlc.arg(now)
FROM media.staged_uploads AS u
WHERE u.id = sqlc.arg(id) AND u.state = 'READY' AND u.expires_at > sqlc.arg(now)
ON CONFLICT DO NOTHING
RETURNING asset_id;

-- name: GetClaim :one
-- 暫存列的認領；here 表示是目前這個 transaction 寫的（Confirm 重入只在同一個 transaction 內冪等，B1-D23）。
SELECT upload_id, kind, asset_id, (claimed_xid = pg_current_xact_id())::boolean AS here
FROM media.upload_claims WHERE upload_id = sqlc.arg(upload_id);

-- name: GetUploadView :one
-- 暫存列加它是否有 CONFIRMED 認領（推導出的 CONFIRMED，不論列上的 state 是 READY 還是已物化的 CONFIRMED）。
SELECT u.*, EXISTS (
    SELECT 1 FROM media.upload_claims AS c WHERE c.upload_id = u.id AND c.kind = 'CONFIRMED') AS confirmed
FROM media.staged_uploads AS u WHERE u.id = sqlc.arg(id);

-- name: ListReclaimableUploads :many
-- GC 階段一的候選：過期且未確認（PENDING、UPLOADING、READY、REJECTED）的列，以及過期超過寬限仍停在 UPLOADED 的列（處理 job 已不會再來），
-- 而且還沒有任何認領。純 SELECT、不鎖列；(expires_at, id) 是 keyset，被跳過的列不會讓迴圈原地打轉。
SELECT u.id, u.expires_at FROM media.staged_uploads AS u
WHERE ((u.state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED') AND u.expires_at <= sqlc.arg(now))
    OR (u.state = 'UPLOADED' AND u.expires_at <= sqlc.arg(uploaded_before)))
  AND (u.expires_at, u.id) > (sqlc.arg(after_expires_at)::timestamptz, sqlc.arg(after_id)::uuid)
  AND NOT EXISTS (SELECT 1 FROM media.upload_claims AS c WHERE c.upload_id = u.id)
ORDER BY u.expires_at, u.id
LIMIT sqlc.arg(max_rows);

-- name: ReclaimUpload :one
-- GC 階段一：以 RECLAIMED 認領一筆過期的暫存列。條件與 ListReclaimableUploads 相同，在插入時重新檢查；輸給 Confirm（或已被認領）沒有列。
-- 呼叫端先 SET LOCAL lock_timeout：同一個鍵被擁有者還沒提交的認領佔著時不等它，跳過這一列。
INSERT INTO media.upload_claims (upload_id, kind, asset_id, created_at)
SELECT u.id, 'RECLAIMED', u.asset_id, sqlc.arg(now)
FROM media.staged_uploads AS u
WHERE u.id = sqlc.arg(id)
  AND ((u.state IN ('PENDING', 'UPLOADING', 'READY', 'REJECTED') AND u.expires_at <= sqlc.arg(now))
    OR (u.state = 'UPLOADED' AND u.expires_at <= sqlc.arg(uploaded_before)))
ON CONFLICT DO NOTHING
RETURNING upload_id;

-- name: MarkReclaimedUploadDeleting :one
-- 贏得 RECLAIMED 認領的暫存列轉 DELETING，回傳它指著的資產（可能沒有）；物件隨後在 transaction 外刪。
UPDATE media.staged_uploads
SET state = 'DELETING', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state IN ('PENDING', 'UPLOADING', 'UPLOADED', 'READY', 'REJECTED')
RETURNING asset_id;

-- name: ListDeletingUploads :many
SELECT id, tmp_key, final_key, asset_id, planned_asset_id FROM media.staged_uploads
WHERE state = 'DELETING' AND id > sqlc.arg(after_id)
ORDER BY id
LIMIT sqlc.arg(max_rows);

-- name: MarkUploadDeleted :execrows
UPDATE media.staged_uploads SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id) AND state = 'DELETING';

-- name: DeleteClaimOfUpload :execrows
-- 暫存列物理刪除完成：它的 RECLAIMED 認領一併刪（state 已是 DELETED，不再需要認領來擋 Confirm）。
DELETE FROM media.upload_claims WHERE upload_id = sqlc.arg(upload_id) AND kind = 'RECLAIMED';

-- name: SettleConfirmedUploads :execrows
-- settle：把已有 CONFIRMED 認領的 READY 暫存列物化成 CONFIRMED，讓 staged_uploads_gc 索引不累積已確認的列。推導不依賴它。
UPDATE media.staged_uploads AS u SET state = 'CONFIRMED', updated_at = sqlc.arg(now)
WHERE u.id IN (
    SELECT u2.id FROM media.staged_uploads AS u2
    JOIN media.upload_claims AS c ON c.upload_id = u2.id AND c.kind = 'CONFIRMED'
    WHERE u2.state = 'READY'
    ORDER BY u2.id
    LIMIT sqlc.arg(max_rows)
    FOR NO KEY UPDATE OF u2 SKIP LOCKED)
  AND u.state = 'READY';
