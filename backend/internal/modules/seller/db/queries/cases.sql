-- name: InsertCase :one
INSERT INTO seller.cases (id, store_id, category, severity, opened_at, opened_by_admin_id, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(category), sqlc.arg(severity), sqlc.arg(now), sqlc.arg(admin_id), sqlc.arg(now))
RETURNING *;

-- name: CaseByID :one
SELECT *
FROM seller.cases
WHERE id = $1;

-- name: LockCase :one
SELECT *
FROM seller.cases
WHERE id = $1
FOR UPDATE;

-- name: LockCases :many
SELECT *
FROM seller.cases
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: LockOpenCase :one
SELECT *
FROM seller.cases
WHERE store_id = sqlc.arg(store_id) AND category = sqlc.arg(category) AND closed_at IS NULL
FOR UPDATE;

-- name: OpenCaseOf :one
SELECT *
FROM seller.cases
WHERE store_id = sqlc.arg(store_id) AND category = sqlc.arg(category) AND closed_at IS NULL;

-- name: OpenCasesOfStore :many
SELECT *
FROM seller.cases
WHERE store_id = sqlc.arg(store_id) AND closed_at IS NULL;

-- name: CaseReportIDs :many
SELECT id
FROM seller.reports
WHERE case_id = $1
ORDER BY created_at, id;

-- name: BumpCase :exec
UPDATE seller.cases
SET version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id);

-- name: DecideCase :one
UPDATE seller.cases
SET closed_at = sqlc.arg(now)::timestamptz,
    closed_by_admin_id = sqlc.arg(admin_id)::bigint,
    close_reason = 'DECIDED',
    decision = sqlc.arg(decision)::text,
    decision_note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)::bigint AND closed_at IS NULL
RETURNING *;

-- name: CloseEmptyCase :one
UPDATE seller.cases c
SET closed_at = sqlc.arg(now)::timestamptz,
    closed_by_admin_id = sqlc.arg(admin_id)::bigint,
    close_reason = 'NO_REPORTS',
    version = c.version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE c.id = sqlc.arg(id) AND c.closed_at IS NULL AND NOT EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id)
RETURNING *;

-- name: ChangeCaseSeverity :one
UPDATE seller.cases
SET severity = sqlc.arg(severity)::text,
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)::bigint AND closed_at IS NULL
RETURNING *;

-- name: CasesByIDs :many
SELECT sqlc.embed(c),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (s.pii_purged_at IS NOT NULL)::bool AS store_purged
FROM seller.cases c
JOIN seller.stores s ON s.store_id = c.store_id
WHERE c.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY c.opened_at DESC, c.id DESC;

-- name: AdminCases :many
SELECT sqlc.embed(c),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (s.pii_purged_at IS NOT NULL)::bool AS store_purged
FROM seller.cases c
JOIN seller.stores s ON s.store_id = c.store_id
WHERE (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'OPEN' AND c.closed_at IS NULL)
    OR (sqlc.narg(state)::text = 'CLOSED' AND c.closed_at IS NOT NULL))
  AND (sqlc.narg(category)::text IS NULL OR c.category = sqlc.narg(category)::text)
  AND (sqlc.narg(store_id)::bigint IS NULL OR c.store_id = sqlc.narg(store_id)::bigint)
  AND (NOT sqlc.arg(awaiting_seller)::bool OR (c.closed_at IS NULL AND EXISTS (
        SELECT 1 FROM seller.case_inquiries i WHERE i.case_id = c.id AND i.reply_received_at IS NULL AND i.withdrawn_at IS NULL)))
  AND (
    sqlc.narg(after_opened_at)::timestamptz IS NULL OR
    (c.opened_at, c.id) < (sqlc.narg(after_opened_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY c.opened_at DESC, c.id DESC
LIMIT sqlc.arg(page_size);

-- name: CaseReportCounts :many
SELECT c.id AS case_id,
       agg.report_count,
       agg.unresolved_count,
       agg.action_taken_count,
       (SELECT count(*) FROM seller.reports r WHERE r.case_id = c.id AND r.outcome IS NULL AND EXISTS (
            SELECT 1 FROM seller.report_supplements p
            WHERE p.report_id = r.id AND p.seq > r.supplements_seen_seq))::int AS unseen_supplement_reports
FROM seller.cases c
CROSS JOIN LATERAL (
    SELECT count(*)::int AS report_count,
           (count(*) FILTER (WHERE r.outcome IS NULL))::int AS unresolved_count,
           (count(*) FILTER (WHERE r.outcome = 'ACTION_TAKEN'))::int AS action_taken_count
    FROM seller.reports r
    WHERE r.case_id = c.id
) agg
WHERE c.id = ANY(sqlc.arg(case_ids)::uuid[]);

-- name: CaseMeasureCounts :many
SELECT d.case_id,
       count(*)::int AS resulting_measures,
       (count(*) FILTER (WHERE d.status = 'ENFORCED'))::int AS unrevoked_measures
FROM (
    SELECT m.source_case_id AS case_id, m.id AS measure_id, m.status
    FROM seller.measures m
    WHERE m.source_case_id = ANY(sqlc.arg(case_ids)::uuid[])
    UNION
    SELECT r.case_id, m.id AS measure_id, m.status
    FROM seller.reports r
    JOIN seller.finding_reports fr ON fr.report_id = r.id
    JOIN seller.measure_findings mf ON mf.violation_id = fr.violation_id
    JOIN seller.measures m ON m.id = mf.measure_id
    WHERE r.case_id = ANY(sqlc.arg(case_ids)::uuid[])
) d
GROUP BY d.case_id;

-- name: InsertReportCaseMove :exec
INSERT INTO seller.report_case_moves (id, report_id, store_id, from_case_id, to_case_id, admin_id, moved_at)
VALUES (sqlc.arg(id), sqlc.arg(report_id), sqlc.arg(store_id), sqlc.arg(from_case_id), sqlc.arg(to_case_id), sqlc.arg(admin_id), sqlc.arg(now));

-- name: CaseMoves :many
SELECT *
FROM seller.report_case_moves
WHERE from_case_id = $1 OR to_case_id = $1
ORDER BY moved_at, id;

-- name: ReviewCaseDecisionByIDs :many
SELECT r.case_id::uuid AS case_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_case_decision(sqlc.arg(at)::timestamptz) r
WHERE r.case_id = ANY(sqlc.arg(ids)::uuid[]);
