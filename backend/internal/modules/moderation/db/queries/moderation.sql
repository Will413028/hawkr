-- name: OpenCaseByTarget :one
-- 對象目前進行中的案件（沒有就是 ErrNoRows）；內容對象帶 content_id、買家對象帶 buyer_id，另一個是 NULL。
SELECT * FROM moderation.cases
WHERE target_kind = sqlc.arg(target_kind) AND closed_at IS NULL
  AND (content_id = sqlc.narg(content_id) OR buyer_id = sqlc.narg(buyer_id));

-- name: InsertContentCase :one
-- 內容對象開案：同一對象已有進行中案件時什麼都不寫（沒有回傳列），呼叫方接著讀進行中的案件（撞唯一鍵用 ON CONFLICT，不在同一個 transaction 繼續下 SQL）。
INSERT INTO moderation.cases (id, target_kind, content_id, opened_at)
VALUES (sqlc.arg(id), sqlc.arg(target_kind), sqlc.arg(content_id), sqlc.arg(opened_at))
ON CONFLICT (target_kind, content_id) WHERE closed_at IS NULL AND content_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertBuyerCase :one
-- 買家對象開案：同 InsertContentCase。
INSERT INTO moderation.cases (id, target_kind, buyer_id, opened_at)
VALUES (sqlc.arg(id), 'BUYER', sqlc.arg(buyer_id), sqlc.arg(opened_at))
ON CONFLICT (buyer_id) WHERE closed_at IS NULL AND buyer_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: AttachToCase :one
-- 新檢舉掛進案件：加一版本並鎖住案件列（同一個案件的結案與其他檢舉的掛入都要等這個 transaction 提交）；回傳 0 列就是案件已被結掉。
UPDATE moderation.cases SET version = version + 1
WHERE id = sqlc.arg(id) AND closed_at IS NULL
RETURNING *;

-- name: GetCase :one
SELECT * FROM moderation.cases WHERE id = sqlc.arg(id);

-- name: ExistingOpenReport :one
-- 這位檢舉人對這個對象在進行中案件裡已有的檢舉。
SELECT r.* FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE c.target_kind = sqlc.arg(target_kind) AND c.closed_at IS NULL
  AND (c.content_id = sqlc.narg(content_id) OR c.buyer_id = sqlc.narg(buyer_id))
  AND r.reporter_buyer_id = sqlc.arg(reporter_buyer_id);

-- name: InsertSnapshot :execrows
-- 同一案同一份內容已有快照時什麼都不寫（0 列）。
INSERT INTO moderation.case_snapshots (id, case_id, content_sha256, content, asset_ids, captured_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(content_sha256), sqlc.arg(content), sqlc.arg(asset_ids)::text[], sqlc.arg(captured_at))
ON CONFLICT (case_id, content_sha256) DO NOTHING;

-- name: SnapshotIDByHash :one
SELECT id FROM moderation.case_snapshots WHERE case_id = sqlc.arg(case_id) AND content_sha256 = sqlc.arg(content_sha256);

-- name: InsertReport :one
-- 同一位檢舉人在同一個案件的第二筆，什麼都不寫（沒有回傳列）。
INSERT INTO moderation.reports (id, case_id, reporter_buyer_id, category, description, snapshot_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(reporter_buyer_id), sqlc.arg(category), sqlc.narg(description), sqlc.arg(snapshot_id), sqlc.arg(created_at))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: InsertReportRequest :execrows
-- 記下 (檢舉人, request_id) 對應的檢舉；同一個 (檢舉人, request_id) 已有紀錄時什麼都不寫（回 0 列，呼叫端重試並比對內容）。
INSERT INTO moderation.report_requests (reporter_buyer_id, request_id, report_id, request_hash)
VALUES (sqlc.arg(reporter_buyer_id), sqlc.arg(request_id), sqlc.arg(report_id), sqlc.arg(request_hash))
ON CONFLICT DO NOTHING;

-- name: ReportByRequest :one
-- 這位檢舉人以這個 request_id 第一次得到的檢舉與當時的內容雜湊（不看案件狀態，A7-D28 第 3 點）。
SELECT sqlc.embed(r), q.request_hash
FROM moderation.report_requests q
JOIN moderation.reports r ON r.id = q.report_id
WHERE q.reporter_buyer_id = sqlc.arg(reporter_buyer_id) AND q.request_id = sqlc.arg(request_id);

-- name: ListReportsOfReporter :many
-- 檢舉人自己的檢舉，新的在前，帶所屬案件的對象與決定（結果由案件推導）。keyset：(created_at, id) 由大到小。
SELECT sqlc.embed(r), c.target_kind, c.content_id, c.buyer_id, c.decision, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.reporter_buyer_id = sqlc.arg(reporter_buyer_id)
  AND (r.created_at, r.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: BumpDailyCount :one
-- 每日上限 20：單一 upsert 原子地換日重算、未達上限才加一；沒有回傳列就是已到上限。DO UPDATE 的 WHERE 讓到上限時不改列。
INSERT INTO moderation.reporter_daily_counts AS c (reporter_buyer_id, day, submitted)
VALUES (sqlc.arg(reporter_buyer_id), sqlc.arg(day), 1)
ON CONFLICT (reporter_buyer_id) DO UPDATE
SET day = EXCLUDED.day,
    submitted = CASE WHEN c.day = EXCLUDED.day THEN c.submitted + 1 ELSE 1 END
WHERE c.day <> EXCLUDED.day OR c.submitted < sqlc.arg(daily_limit)::int
RETURNING c.submitted;

-- name: ReportsOfCase :many
SELECT * FROM moderation.reports WHERE case_id = sqlc.arg(case_id) ORDER BY created_at, id;

-- name: ReportIDsOfCase :many
-- 認定引用的來源檢舉（最早的幾筆）。
SELECT id FROM moderation.reports WHERE case_id = sqlc.arg(case_id) ORDER BY created_at, id LIMIT sqlc.arg(max_rows)::int;

-- name: CountReportsOfCase :one
SELECT count(*) FROM moderation.reports WHERE case_id = sqlc.arg(case_id);

-- name: LatestSnapshotContent :one
-- 案件最近一份凍結的內容（內容已被刪除時用它確認作者）。
SELECT content FROM moderation.case_snapshots WHERE case_id = sqlc.arg(case_id) ORDER BY captured_at DESC, id DESC LIMIT 1;

-- name: ReportWithOutcome :one
-- 一筆檢舉與它所屬案件的決定：檢舉不存結果，結果由案件推導。
SELECT sqlc.embed(r), c.decision, c.decision_category, c.decision_action, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.id = sqlc.arg(id);

-- name: CloseCase :execrows
-- 結案是終態：只有進行中而且版本相符的案件轉移成功；回傳 0 列表示已被結掉或版本過期。
UPDATE moderation.cases
SET decision = sqlc.arg(decision), decision_category = sqlc.narg(decision_category), decision_action = sqlc.narg(decision_action),
    decided_by_admin_id = sqlc.arg(admin_id), closed_at = sqlc.arg(closed_at), version = version + 1
WHERE id = sqlc.arg(id) AND closed_at IS NULL AND version = sqlc.arg(version);

-- name: ExpiredSnapshotsBatch :many
-- 結案早於 cutoff 的案件的快照（與它們引用的資產）：鎖住這些列，同一個 transaction 內釋放資產引用再刪除。
SELECT s.id, s.asset_ids FROM moderation.case_snapshots s
JOIN moderation.cases c ON c.id = s.case_id
WHERE c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
ORDER BY s.id
LIMIT sqlc.arg(max_rows)::int
FOR UPDATE OF s SKIP LOCKED;

-- name: DeleteSnapshots :execrows
-- 刪快照，檢舉的 snapshot_id 由 ON DELETE SET NULL 清空，案件與檢舉列留著。
DELETE FROM moderation.case_snapshots WHERE id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ClearExpiredDescriptionsBatch :execrows
-- 結案早於 cutoff 的案件的檢舉說明（同快照的保留期，A7-D21）。
UPDATE moderation.reports SET description = NULL
WHERE id IN (
    SELECT r.id FROM moderation.reports r
    JOIN moderation.cases c ON c.id = r.case_id
    WHERE r.description IS NOT NULL AND c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
    ORDER BY r.id
    LIMIT sqlc.arg(max_rows)::int
);

-- name: ClearReporterDescriptions :execrows
-- 檢舉人刪帳（A7-D21）：清掉他全部檢舉的說明；檢舉列與 reporter_buyer_id（假名鍵）留著。
UPDATE moderation.reports SET description = NULL WHERE reporter_buyer_id = sqlc.arg(reporter_buyer_id) AND description IS NOT NULL;

-- name: DeleteReporterRequests :execrows
-- 檢舉人刪帳（A7-D21）：刪掉他的冪等紀錄；request_hash 是說明的雜湊，不能比說明活得久。
DELETE FROM moderation.report_requests WHERE reporter_buyer_id = sqlc.arg(reporter_buyer_id);

-- name: DeleteExpiredReportRequestsBatch :execrows
-- 結案早於 cutoff 的案件的檢舉的冪等紀錄（同說明與快照的保留期，A7-D21）。
DELETE FROM moderation.report_requests
WHERE (reporter_buyer_id, request_id) IN (
    SELECT q.reporter_buyer_id, q.request_id FROM moderation.report_requests q
    JOIN moderation.reports r ON r.id = q.report_id
    JOIN moderation.cases c ON c.id = r.case_id
    WHERE c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
    ORDER BY q.reporter_buyer_id, q.request_id
    LIMIT sqlc.arg(max_rows)::int
);

-- name: DeleteReporterDailyCount :execrows
DELETE FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = sqlc.arg(reporter_buyer_id);

-- name: SnapshotContent :one
SELECT content FROM moderation.case_snapshots WHERE id = sqlc.arg(id);

-- name: SnapshotAssets :one
-- 快照目前引用的資產（同步 job 以它為這個 ref 目前的資產）；快照已刪除沒有列。
SELECT asset_ids FROM moderation.case_snapshots WHERE id = sqlc.arg(id);

-- name: RetireAsset :exec
-- 記下被釋放的資產；已經記過的不變。
INSERT INTO moderation.retired_assets (ref_type, ref_id, asset_id, retired_at)
VALUES (sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(asset_id), sqlc.arg(now))
ON CONFLICT (ref_type, ref_id, asset_id) DO NOTHING;

-- name: BumpRefSeq :exec
-- ref 的資產改變了：遞增版本（沒有列時建立）並記下變動時間。與同步 job 的結算比對同一列，互相等待。
INSERT INTO moderation.media_ref_sync (ref_type, ref_id, updated_at)
VALUES (sqlc.arg(ref_type), sqlc.arg(ref_id), sqlc.arg(now))
ON CONFLICT (ref_type, ref_id) DO UPDATE SET seq = moderation.media_ref_sync.seq + 1, updated_at = EXCLUDED.updated_at;

-- name: RefSeq :one
SELECT seq FROM moderation.media_ref_sync WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id);

-- name: RetiredAssetIDs :many
SELECT asset_id FROM moderation.retired_assets
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id)
ORDER BY asset_id;

-- name: RetiredAmong :many
SELECT asset_id FROM moderation.retired_assets
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND asset_id = ANY(sqlc.arg(asset_ids)::text[])
ORDER BY asset_id;

-- name: MarkUnlinked :exec
UPDATE moderation.retired_assets
SET unlinked_at = COALESCE(unlinked_at, sqlc.arg(now))
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND asset_id = ANY(sqlc.arg(asset_ids)::text[]);

-- name: PurgeRetired :exec
-- 清掉已經過第二次解除、而且超過 staged token 有效期的退場紀錄（整個模組）。
DELETE FROM moderation.retired_assets
WHERE unlinked_at < sqlc.arg(heal_before) AND retired_at < sqlc.arg(retire_before);

-- name: SettleRefSeq :execrows
-- 只在版本還是同步讀到的那個時記下已同步；0 列表示讀取之後 ref 又改變了。
UPDATE moderation.media_ref_sync
SET synced_seq = sqlc.arg(seq)
WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND seq = sqlc.arg(seq);

-- name: HealingRetired :one
SELECT EXISTS (
    SELECT 1 FROM moderation.retired_assets
    WHERE ref_type = sqlc.arg(ref_type) AND ref_id = sqlc.arg(ref_id) AND unlinked_at >= sqlc.arg(heal_before)
);

-- name: UnsyncedRefs :many
-- 版本比已同步的新、而且最後變動早於 idle_before 的 ref，最舊的先（保底掃描補排用）。
SELECT ref_type, ref_id, updated_at
FROM moderation.media_ref_sync
WHERE synced_seq < seq AND updated_at < sqlc.arg(idle_before)
ORDER BY updated_at
LIMIT sqlc.arg(max_rows);
