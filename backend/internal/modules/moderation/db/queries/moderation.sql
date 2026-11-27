-- name: OpenCaseByTarget :one
SELECT * FROM moderation.cases
WHERE target_kind = sqlc.arg(target_kind) AND closed_at IS NULL
  AND (content_id = sqlc.narg(content_id) OR buyer_id = sqlc.narg(buyer_id));

-- name: InsertContentCase :one
INSERT INTO moderation.cases (id, target_kind, content_id, opened_at)
VALUES (sqlc.arg(id), sqlc.arg(target_kind), sqlc.arg(content_id), sqlc.arg(opened_at))
ON CONFLICT (target_kind, content_id) WHERE closed_at IS NULL AND content_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertBuyerCase :one
INSERT INTO moderation.cases (id, target_kind, buyer_id, opened_at)
VALUES (sqlc.arg(id), 'BUYER', sqlc.arg(buyer_id), sqlc.arg(opened_at))
ON CONFLICT (buyer_id) WHERE closed_at IS NULL AND buyer_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: AttachToCase :one
UPDATE moderation.cases SET version = version + 1
WHERE id = sqlc.arg(id) AND closed_at IS NULL
RETURNING *;

-- name: GetCase :one
SELECT * FROM moderation.cases WHERE id = sqlc.arg(id);

-- name: ExistingOpenReport :one
SELECT r.* FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE c.target_kind = sqlc.arg(target_kind) AND c.closed_at IS NULL
  AND (c.content_id = sqlc.narg(content_id) OR c.buyer_id = sqlc.narg(buyer_id))
  AND (r.reporter_buyer_id = sqlc.narg(reporter_buyer_id) OR r.reporter_store_id = sqlc.narg(reporter_store_id));

-- name: InsertSnapshot :execrows
INSERT INTO moderation.case_snapshots (id, case_id, content_sha256, content, asset_ids, captured_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(content_sha256), sqlc.arg(content), sqlc.arg(asset_ids)::uuid[], sqlc.arg(captured_at))
ON CONFLICT (case_id, content_sha256) DO NOTHING;

-- name: SnapshotIDByHash :one
SELECT id FROM moderation.case_snapshots WHERE case_id = sqlc.arg(case_id) AND content_sha256 = sqlc.arg(content_sha256);

-- name: InsertReport :one
INSERT INTO moderation.reports (id, case_id, reporter_buyer_id, reporter_store_id, category, description, snapshot_id, created_at)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.narg(reporter_buyer_id), sqlc.narg(reporter_store_id), sqlc.arg(category), sqlc.narg(description), sqlc.arg(snapshot_id), sqlc.arg(created_at))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: InsertReportRequest :execrows
INSERT INTO moderation.report_requests (reporter_buyer_id, reporter_store_id, request_id, report_id, request_hash)
VALUES (sqlc.narg(reporter_buyer_id), sqlc.narg(reporter_store_id), sqlc.arg(request_id), sqlc.arg(report_id), sqlc.arg(request_hash))
ON CONFLICT DO NOTHING;

-- name: ReportByRequest :one
SELECT sqlc.embed(r), q.request_hash
FROM moderation.report_requests q
JOIN moderation.reports r ON r.id = q.report_id
WHERE (q.reporter_buyer_id = sqlc.narg(reporter_buyer_id) OR q.reporter_store_id = sqlc.narg(reporter_store_id)) AND q.request_id = sqlc.arg(request_id);

-- name: ListReportsOfReporter :many
SELECT sqlc.embed(r), c.target_kind, c.content_id, c.buyer_id, c.decision, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.reporter_buyer_id = sqlc.arg(reporter_buyer_id)
  AND (r.created_at, r.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListReportsOfStoreReporter :many
SELECT sqlc.embed(r), c.target_kind, c.content_id, c.buyer_id, c.decision, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.reporter_store_id = sqlc.arg(reporter_store_id)
  AND (r.created_at, r.id) < (sqlc.arg(after_created)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: BumpDailyCount :one
INSERT INTO moderation.reporter_daily_counts AS c (reporter_buyer_id, day, submitted)
VALUES (sqlc.arg(reporter_buyer_id), sqlc.arg(day), 1)
ON CONFLICT (reporter_buyer_id) DO UPDATE
SET day = EXCLUDED.day,
    submitted = CASE WHEN c.day = EXCLUDED.day THEN c.submitted + 1 ELSE 1 END
WHERE c.day <> EXCLUDED.day OR c.submitted < sqlc.arg(daily_limit)::int
RETURNING c.submitted;

-- name: BumpStoreDailyCount :one
INSERT INTO moderation.reporter_daily_counts AS c (reporter_store_id, day, submitted)
VALUES (sqlc.arg(reporter_store_id), sqlc.arg(day), 1)
ON CONFLICT (reporter_store_id) DO UPDATE
SET day = EXCLUDED.day,
    submitted = CASE WHEN c.day = EXCLUDED.day THEN c.submitted + 1 ELSE 1 END
WHERE c.day <> EXCLUDED.day OR c.submitted < sqlc.arg(daily_limit)::int
RETURNING c.submitted;

-- name: ReportsOfCase :many
SELECT * FROM moderation.reports WHERE case_id = sqlc.arg(case_id) ORDER BY created_at, id;

-- name: BuyerReportersOfCase :many
SELECT id, reporter_buyer_id::bigint AS reporter_id FROM moderation.reports
WHERE case_id = sqlc.arg(case_id) AND reporter_buyer_id > sqlc.arg(after_id)::bigint
ORDER BY reporter_buyer_id
LIMIT sqlc.arg(max_rows)::int;

-- name: StoreReportersOfCase :many
SELECT id, reporter_store_id::bigint AS reporter_id FROM moderation.reports
WHERE case_id = sqlc.arg(case_id) AND reporter_store_id > sqlc.arg(after_id)::bigint
ORDER BY reporter_store_id
LIMIT sqlc.arg(max_rows)::int;

-- name: ReportIDsOfCase :many
SELECT id FROM moderation.reports WHERE case_id = sqlc.arg(case_id) ORDER BY created_at, id LIMIT sqlc.arg(max_rows)::int;

-- name: CountReportsOfCase :one
SELECT count(*) FROM moderation.reports WHERE case_id = sqlc.arg(case_id);

-- name: LatestSnapshotContent :one
SELECT content FROM moderation.case_snapshots WHERE case_id = sqlc.arg(case_id) ORDER BY captured_at DESC, id DESC LIMIT 1;

-- name: ReportWithOutcome :one
SELECT sqlc.embed(r), c.decision, c.decision_category, c.decision_action, c.closed_at
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE r.id = sqlc.arg(id);

-- name: CloseCase :execrows
UPDATE moderation.cases
SET decision = sqlc.arg(decision), decision_category = sqlc.narg(decision_category), decision_action = sqlc.narg(decision_action),
    decided_by_admin_id = sqlc.arg(admin_id), closed_at = sqlc.arg(closed_at), measure_id = sqlc.narg(measure_id), version = version + 1
WHERE id = sqlc.arg(id) AND closed_at IS NULL AND version = sqlc.arg(version);

-- name: ReverseCase :execrows
UPDATE moderation.cases
SET reversed_at = sqlc.arg(reversed_at), reversed_by_admin_id = sqlc.arg(admin_id), reverse_reason = sqlc.arg(reason)
WHERE id = sqlc.arg(id) AND closed_at IS NOT NULL AND decision_action = 'HIDE' AND reversed_at IS NULL;

-- name: ExpiredSnapshotsBatch :many
SELECT s.id FROM moderation.case_snapshots s
JOIN moderation.cases c ON c.id = s.case_id
WHERE c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
ORDER BY s.id
LIMIT sqlc.arg(max_rows)::int
FOR UPDATE OF s SKIP LOCKED;

-- name: DeleteSnapshots :many
DELETE FROM moderation.case_snapshots WHERE id = ANY(sqlc.arg(ids)::uuid[]) RETURNING asset_ids;

-- name: ClearExpiredDescriptionsBatch :execrows
UPDATE moderation.reports SET description = NULL
WHERE id IN (
    SELECT r.id FROM moderation.reports r
    JOIN moderation.cases c ON c.id = r.case_id
    WHERE r.description IS NOT NULL AND c.closed_at IS NOT NULL AND c.closed_at < sqlc.arg(cutoff)
    ORDER BY r.id
    LIMIT sqlc.arg(max_rows)::int
);

-- name: ClearReporterDescriptions :execrows
UPDATE moderation.reports SET description = NULL WHERE reporter_buyer_id = sqlc.arg(reporter_buyer_id) AND description IS NOT NULL;

-- name: DeleteReporterRequests :execrows
DELETE FROM moderation.report_requests WHERE reporter_buyer_id = sqlc.arg(reporter_buyer_id);

-- name: DeleteExpiredReportRequestsBatch :execrows
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
UPDATE moderation.reports SET description = NULL WHERE reporter_store_id = sqlc.arg(reporter_store_id) AND description IS NOT NULL;

-- name: DeleteStoreReporterRequests :execrows
DELETE FROM moderation.report_requests WHERE reporter_store_id = sqlc.arg(reporter_store_id);

-- name: DeleteStoreReporterDailyCount :execrows
DELETE FROM moderation.reporter_daily_counts WHERE reporter_store_id = sqlc.arg(reporter_store_id);

-- name: SnapshotContent :one
SELECT content FROM moderation.case_snapshots WHERE id = sqlc.arg(id);

-- name: ListOpenCases :many
SELECT *
FROM moderation.cases
WHERE closed_at IS NULL
  AND (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (opened_at, id) < (sqlc.arg(after_opened)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListClosedCases :many
SELECT *
FROM moderation.cases
WHERE closed_at IS NOT NULL
  AND (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (opened_at, id) < (sqlc.arg(after_opened)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListAllCases :many
SELECT *
FROM moderation.cases
WHERE (sqlc.arg(target_kind)::text = '' OR target_kind = sqlc.arg(target_kind)::text)
  AND (opened_at, id) < (sqlc.arg(after_opened)::timestamptz, sqlc.arg(after_id)::uuid)
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: OpenReportCountsOfPosts :many
SELECT c.content_id AS post_id, count(*)::bigint AS reports
FROM moderation.reports r
JOIN moderation.cases c ON c.id = r.case_id
WHERE c.target_kind = 'POST' AND c.closed_at IS NULL AND c.content_id = ANY(sqlc.arg(post_ids)::uuid[])
GROUP BY c.content_id;

-- name: ReportCountsOfCases :many
SELECT case_id, count(*)::bigint AS reports
FROM moderation.reports
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[])
GROUP BY case_id;

-- name: LatestSnapshotContentsOfCases :many
SELECT DISTINCT ON (case_id) case_id, content
FROM moderation.case_snapshots
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[])
ORDER BY case_id, captured_at DESC, id DESC;
