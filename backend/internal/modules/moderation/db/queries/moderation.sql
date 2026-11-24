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
-- 這位檢舉人（買家或店家，另一欄傳 NULL）對這個對象在進行中案件裡已有的檢舉。
SELECT r.* FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE c.target_kind = sqlc.arg(target_kind) AND c.closed_at IS NULL
  AND (c.content_id = sqlc.narg(content_id) OR c.buyer_id = sqlc.narg(buyer_id))
  AND (r.reporter_buyer_id = sqlc.narg(reporter_buyer_id) OR r.reporter_store_id = sqlc.narg(reporter_store_id));

-- name: InsertSnapshot :execrows
-- 同一案同一份內容已有快照時什麼都不寫（0 列）。
INSERT INTO moderation.case_snapshots (id, case_id, content_sha256, content, asset_ids, captured_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(content_sha256), sqlc.arg(content), sqlc.arg(asset_ids)::uuid[], sqlc.arg(captured_at))
ON CONFLICT (case_id, content_sha256) DO NOTHING;

-- name: SnapshotIDByHash :one
SELECT id FROM moderation.case_snapshots WHERE case_id = sqlc.arg(case_id) AND content_sha256 = sqlc.arg(content_sha256);

-- name: InsertReport :one
-- 同一位檢舉人在同一個案件的第二筆，什麼都不寫（沒有回傳列）。
INSERT INTO moderation.reports (id, case_id, reporter_buyer_id, reporter_store_id, category, description, snapshot_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.narg(reporter_buyer_id), sqlc.narg(reporter_store_id), sqlc.arg(category), sqlc.narg(description), sqlc.arg(snapshot_id), sqlc.arg(created_at))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: InsertReportRequest :execrows
-- 記下 (檢舉人, request_id) 對應的檢舉；同一個 (檢舉人, request_id) 已有紀錄時什麼都不寫（回 0 列，呼叫端重試並比對內容）。
INSERT INTO moderation.report_requests (reporter_buyer_id, reporter_store_id, request_id, report_id, request_hash)
VALUES (sqlc.narg(reporter_buyer_id), sqlc.narg(reporter_store_id), sqlc.arg(request_id), sqlc.arg(report_id), sqlc.arg(request_hash))
ON CONFLICT DO NOTHING;

-- name: ReportByRequest :one
-- 這位檢舉人以這個 request_id 第一次得到的檢舉與當時的內容雜湊（不看案件狀態，A7-D28 第 3 點）。
SELECT sqlc.embed(r), q.request_hash
FROM moderation.report_requests q
JOIN moderation.reports r ON r.id = q.report_id
WHERE (q.reporter_buyer_id = sqlc.narg(reporter_buyer_id) OR q.reporter_store_id = sqlc.narg(reporter_store_id)) AND q.request_id = sqlc.arg(request_id);

-- name: ListReportsOfReporter :many
-- 檢舉人自己的檢舉，新的在前，帶所屬案件的對象與決定（結果由案件推導）。keyset：(created_at, id) 由大到小。
SELECT sqlc.embed(r), c.target_kind, c.content_id, c.buyer_id, c.decision, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.reporter_buyer_id = sqlc.arg(reporter_buyer_id)
  AND (r.created_at, r.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListReportsOfStoreReporter :many
-- 店家檢舉人自己的檢舉（只有聊天訊息），形狀與 ListReportsOfReporter 相同。keyset：(created_at, id) 由大到小。
SELECT sqlc.embed(r), c.target_kind, c.content_id, c.buyer_id, c.decision, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.reporter_store_id = sqlc.arg(reporter_store_id)
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

-- name: BumpStoreDailyCount :one
-- 店家檢舉人的每日上限，與買家各自計數，規則同 BumpDailyCount。
INSERT INTO moderation.reporter_daily_counts AS c (reporter_store_id, day, submitted)
VALUES (sqlc.arg(reporter_store_id), sqlc.arg(day), 1)
ON CONFLICT (reporter_store_id) DO UPDATE
SET day = EXCLUDED.day,
    submitted = CASE WHEN c.day = EXCLUDED.day THEN c.submitted + 1 ELSE 1 END
WHERE c.day <> EXCLUDED.day OR c.submitted < sqlc.arg(daily_limit)::int
RETURNING c.submitted;

-- name: ReportsOfCase :many
SELECT * FROM moderation.reports WHERE case_id = sqlc.arg(case_id) ORDER BY created_at, id;

-- name: ReportersOfCase :many
-- 案件的檢舉人，依 (created_at, id) 由小到大的 keyset；第一頁的游標是零值。
SELECT id, created_at, reporter_buyer_id, reporter_store_id FROM moderation.reports
WHERE case_id = sqlc.arg(case_id) AND (created_at, id) > (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY created_at, id
LIMIT sqlc.arg(max_rows)::int;

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
    decided_by_admin_id = sqlc.arg(admin_id), closed_at = sqlc.arg(closed_at), measure_id = sqlc.narg(measure_id), version = version + 1
WHERE id = sqlc.arg(id) AND closed_at IS NULL AND version = sqlc.arg(version);

-- name: ReverseCase :execrows
-- 撤銷 HIDE 決定：案件維持結案、決定不改，撤銷記在原決定旁。只有已結案、處置是 HIDE、還沒撤銷的案件轉移成功；0 列表示不符合（已被撤銷或不是 HIDE）。
UPDATE moderation.cases
SET reversed_at = sqlc.arg(reversed_at), reversed_by_admin_id = sqlc.arg(admin_id), reverse_reason = sqlc.arg(reason)
WHERE id = sqlc.arg(id) AND closed_at IS NOT NULL AND decision_action = 'HIDE' AND reversed_at IS NULL;

-- name: ExpiredSnapshotsBatch :many
-- 結案早於 cutoff 的案件的快照：鎖住這些列，同一個 transaction 內刪除。
SELECT s.id FROM moderation.case_snapshots s
JOIN moderation.cases c ON c.id = s.case_id
WHERE c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
ORDER BY s.id
LIMIT sqlc.arg(max_rows)::int
FOR UPDATE OF s SKIP LOCKED;

-- name: DeleteSnapshots :many
-- 刪快照並回傳各自擁有的圖片資產（呼叫方在同一個 transaction 內 MarkDeleted）；檢舉的 snapshot_id 由 ON DELETE SET NULL 清空，案件與檢舉列留著。
DELETE FROM moderation.case_snapshots WHERE id = ANY(sqlc.arg(ids)::uuid[]) RETURNING asset_ids;

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
WHERE report_id IN (
    SELECT q.report_id
    FROM moderation.report_requests q
    JOIN moderation.reports r ON r.id = q.report_id
    JOIN moderation.cases c ON c.id = r.case_id
    WHERE c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
    LIMIT sqlc.arg(max_rows)::int
);

-- name: DeleteReporterDailyCount :execrows
DELETE FROM moderation.reporter_daily_counts WHERE reporter_buyer_id = sqlc.arg(reporter_buyer_id);

-- name: ClearStoreReporterDescriptions :execrows
-- 店家退場：清掉該店以檢舉人身分送出的全部檢舉的說明；檢舉列與 reporter_store_id（業務鍵）留著。
UPDATE moderation.reports SET description = NULL WHERE reporter_store_id = sqlc.arg(reporter_store_id) AND description IS NOT NULL;

-- name: DeleteStoreReporterRequests :execrows
-- 店家退場：刪掉該店的冪等紀錄；request_hash 是說明的雜湊，不能比說明活得久。
DELETE FROM moderation.report_requests WHERE reporter_store_id = sqlc.arg(reporter_store_id);

-- name: DeleteStoreReporterDailyCount :execrows
DELETE FROM moderation.reporter_daily_counts WHERE reporter_store_id = sqlc.arg(reporter_store_id);

-- name: SnapshotContent :one
SELECT content FROM moderation.case_snapshots WHERE id = sqlc.arg(id);

-- 列案件依狀態分三個查詢：狀態是述詞的常數而不是參數，OPEN 才證明得出 partial index cases_open_by_time 適用（參數化的 catch-all 述詞在 generic plan 下證明不了）。
-- name: ListOpenCases :many
-- 後台列案件，開案時間新的在前（keyset：(opened_at, id) 由大到小）；進行中的案件；target_kind 空字串不篩。
SELECT *
FROM moderation.cases
WHERE closed_at IS NULL
  AND (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (opened_at, id) < (sqlc.arg(after_opened)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListClosedCases :many
-- 後台列案件，開案時間新的在前（keyset：(opened_at, id) 由大到小）；已結案的案件；target_kind 空字串不篩。
SELECT *
FROM moderation.cases
WHERE closed_at IS NOT NULL
  AND (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (opened_at, id) < (sqlc.arg(after_opened)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListAllCases :many
-- 後台列案件，開案時間新的在前（keyset：(opened_at, id) 由大到小）；不分狀態；target_kind 空字串不篩。
SELECT *
FROM moderation.cases
WHERE (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (opened_at, id) < (sqlc.arg(after_opened)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: OpenReportCountsOfPosts :many
-- 貼文在進行中案件裡的檢舉數（已結案案件的檢舉不算）；沒有進行中案件的貼文沒有列。
SELECT c.content_id AS post_id, count(*)::bigint AS reports
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE c.target_kind = 'POST' AND c.closed_at IS NULL AND c.content_id = ANY(sqlc.arg(post_ids)::uuid[])
GROUP BY c.content_id;

-- name: ReportCountsOfCases :many
-- 一頁案件的檢舉數（一個查詢，不逐案件查）。
SELECT case_id, count(*)::bigint AS reports
FROM moderation.reports
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[])
GROUP BY case_id;

-- name: LatestSnapshotContentsOfCases :many
-- 一頁案件各自最近一份凍結的內容；沒有快照（已過保留期被刪除）的案件沒有列。
SELECT DISTINCT ON (case_id) case_id, content
FROM moderation.case_snapshots
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[])
ORDER BY case_id, captured_at DESC, id DESC;
