-- name: RetireAsset :exec
-- 記下被換掉的資產；已經記過的不變。
INSERT INTO seller.retired_assets (ref_type, ref_id, asset_id, retired_at)
VALUES (sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(asset_id), sqlc.arg(now))
ON CONFLICT (ref_type, ref_id, asset_id) DO NOTHING;

-- name: BumpRefSeq :exec
-- 業務列改變了 ref 的資產：遞增版本（沒有列時建立）並記下變動時間。與同步 job 的結算比對同一列，互相等待。
INSERT INTO seller.media_ref_sync (ref_type, ref_id, updated_at)
VALUES (sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(now))
ON CONFLICT (ref_type, ref_id) DO UPDATE SET seq = seller.media_ref_sync.seq + 1, updated_at = EXCLUDED.updated_at;

-- name: RefSeq :one
SELECT seq FROM seller.media_ref_sync WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id);

-- name: RetiredAssetIDs :many
-- ref 還沒保留期滿的退場資產。
SELECT asset_id FROM seller.retired_assets
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id)
ORDER BY asset_id;

-- name: RetiredAmong :many
-- assets 之中已經在這個 ref 退場的。
SELECT asset_id FROM seller.retired_assets
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND asset_id = ANY(sqlc.arg(asset_ids)::text[])
ORDER BY asset_id;

-- name: MarkUnlinked :exec
UPDATE seller.retired_assets
SET unlinked_at = COALESCE(unlinked_at, sqlc.arg(now))
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND asset_id = ANY(sqlc.arg(asset_ids)::text[]);

-- name: PurgeRetired :exec
-- 清掉已經過第二次解除、而且超過 staged token 有效期的退場紀錄（整個模組）。
DELETE FROM seller.retired_assets
WHERE unlinked_at < sqlc.arg(heal_before) AND retired_at < sqlc.arg(retire_before);

-- name: SettleRefSeq :execrows
-- 只在版本還是同步讀到的那個時記下已同步；0 列表示讀取之後業務列又改變了。
UPDATE seller.media_ref_sync
SET synced_seq = sqlc.arg(seq)
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND seq = sqlc.arg(seq);

-- name: HealingRetired :one
-- ref 是否還有第一次解除不到 heal_delay 的退場資產。
SELECT EXISTS (
    SELECT 1 FROM seller.retired_assets
    WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND unlinked_at >= sqlc.arg(heal_before)
);

-- name: UnsyncedRefs :many
-- 版本比已同步的新、而且最後變動早於 idle_before 的 ref，最舊的先（保底掃描補排用）。
SELECT ref_type, ref_id, updated_at
FROM seller.media_ref_sync
WHERE synced_seq < seq AND updated_at < sqlc.arg(idle_before)
ORDER BY updated_at
LIMIT sqlc.arg(max_rows);

-- name: MediaSyncSettled :one
-- ref 的引用在 media 的同步是否已經結算（synced_seq 追上 seq）；還沒有同步列時算沒結算。
SELECT COALESCE(
    (SELECT synced_seq >= seq FROM seller.media_ref_sync WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id)),
    false
)::boolean AS settled;
