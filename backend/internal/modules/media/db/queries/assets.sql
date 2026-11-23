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

-- name: GetAssetView :one
-- 資產加推導狀態需要的兩個事實（B1-D25）：有沒有 CONFIRMED 認領、有沒有刪除請求；model.DeriveAssetState 由它們與 state 推導。
SELECT a.*,
    EXISTS (SELECT 1 FROM media.upload_claims AS c WHERE c.asset_id = a.id AND c.kind = 'CONFIRMED') AS confirmed,
    EXISTS (SELECT 1 FROM media.asset_deletion_requests AS r WHERE r.asset_id = a.id) AS delete_requested
FROM media.assets AS a WHERE a.id = sqlc.arg(id);

-- name: ListAssetIDs :many
-- 診斷與測試：列出所有資產 ID（狀態由 GetAssetView 推導，不在這裡重寫謂詞）。
SELECT id FROM media.assets ORDER BY id;

-- name: RequestDeletions :many
-- 擁有者的刪除指令（B1-D25）：只對已確認的資產 INSERT 請求列，不更新任何既有列；未確認、不存在的 ID 被濾掉，已請求過的冪等。
INSERT INTO media.asset_deletion_requests (asset_id, requested_at)
SELECT c.asset_id, sqlc.arg(now) FROM media.upload_claims AS c
WHERE c.asset_id = ANY(sqlc.arg(ids)::uuid[]) AND c.kind = 'CONFIRMED'
ORDER BY c.asset_id
ON CONFLICT (asset_id) DO NOTHING
RETURNING asset_id;

-- name: ApplyDeletionRequests :many
-- 套用 job：把還沒套用的請求所指的資產標為 DELETING，並記 applied_at。冪等：資產只從 STAGED 轉，applied_at 只寫一次，兩個 worker 由 SKIP LOCKED 分開。
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
-- GC 階段一的另一半：贏得 RECLAIMED 認領的暫存列所指的資產（還是 STAGED）一起轉 DELETING。
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

-- name: SettleDeletedAssetUploads :execrows
-- 資產物理刪除完成：認領它的暫存列（擁有者確認的那一筆）轉 DELETED，之後它的 CONFIRMED 認領可以刪。
UPDATE media.staged_uploads SET state = 'DELETED', updated_at = sqlc.arg(now)
WHERE id IN (SELECT c.upload_id FROM media.upload_claims AS c WHERE c.asset_id = sqlc.arg(asset_id)::uuid AND c.kind = 'CONFIRMED')
  AND state IN ('READY', 'CONFIRMED');

-- name: DeleteClaimsOfAsset :execrows
DELETE FROM media.upload_claims WHERE asset_id = sqlc.arg(asset_id)::uuid;

-- name: DeleteRequestOfAsset :execrows
DELETE FROM media.asset_deletion_requests WHERE asset_id = sqlc.arg(asset_id);
