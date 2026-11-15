-- name: OpenCaseByTarget :one
-- 對象目前進行中的案件（沒有就是 ErrNoRows）。
SELECT * FROM moderation.cases
WHERE target_kind = sqlc.arg(target_kind) AND target_key = sqlc.arg(target_key) AND closed_at IS NULL;

-- name: InsertCase :one
-- 開案：同一對象已有進行中案件時什麼都不寫（沒有回傳列），呼叫方接著讀進行中的案件（撞唯一鍵用 ON CONFLICT，不在同一個 transaction 繼續下 SQL）。
INSERT INTO moderation.cases (id, target_kind, content_id, buyer_id, opened_at)
VALUES (sqlc.arg(id), sqlc.arg(target_kind), sqlc.narg(content_id), sqlc.narg(buyer_id), sqlc.arg(opened_at))
ON CONFLICT (target_kind, target_key) WHERE closed_at IS NULL DO NOTHING
RETURNING *;

-- name: ShareLockCase :one
-- 共享鎖住案件：同一個案件的結案（UPDATE）要等這個 transaction 提交，所以掛上檢舉之後案件不會在中間被結掉；呼叫方檢查 closed_at。
SELECT * FROM moderation.cases WHERE id = sqlc.arg(id) FOR SHARE;

-- name: GetCase :one
SELECT * FROM moderation.cases WHERE id = sqlc.arg(id);

-- name: ExistingOpenReport :one
-- 這位檢舉人對這個對象在進行中案件裡已有的檢舉。
SELECT r.* FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE c.target_kind = sqlc.arg(target_kind) AND c.target_key = sqlc.arg(target_key) AND c.closed_at IS NULL
  AND r.reporter_buyer_id = sqlc.arg(reporter_buyer_id);

-- name: InsertSnapshot :exec
INSERT INTO moderation.case_snapshots (id, case_id, content_sha256, content, captured_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(content_sha256), sqlc.arg(content), sqlc.arg(captured_at))
ON CONFLICT (case_id, content_sha256) DO NOTHING;

-- name: SnapshotIDByHash :one
SELECT id FROM moderation.case_snapshots WHERE case_id = sqlc.arg(case_id) AND content_sha256 = sqlc.arg(content_sha256);

-- name: InsertReport :one
-- 同一位檢舉人在同一個案件的第二筆什麼都不寫（沒有回傳列）。
INSERT INTO moderation.reports (id, case_id, reporter_buyer_id, category, description, snapshot_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(reporter_buyer_id), sqlc.arg(category), sqlc.narg(description), sqlc.arg(snapshot_id), sqlc.arg(created_at))
ON CONFLICT (case_id, reporter_buyer_id) DO NOTHING
RETURNING *;

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

-- name: PurgeSnapshotsBatch :execrows
-- 結案早於 cutoff 的案件的快照：刪快照，檢舉的 snapshot_id 由 ON DELETE SET NULL 清空，案件與檢舉列留著。
DELETE FROM moderation.case_snapshots
WHERE id IN (
    SELECT s.id FROM moderation.case_snapshots s
    JOIN moderation.cases c ON c.id = s.case_id
    WHERE c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
    ORDER BY s.id
    LIMIT sqlc.arg(max_rows)::int
);

-- name: SnapshotContent :one
SELECT content FROM moderation.case_snapshots WHERE id = sqlc.arg(id);
