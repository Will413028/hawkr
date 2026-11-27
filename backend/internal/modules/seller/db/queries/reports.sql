-- name: InsertReport :one
INSERT INTO seller.reports (
    id, store_id, product_id, order_no, category, description, reporter_buyer_id,
    created_at, updated_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.narg(product_id), sqlc.narg(order_no), sqlc.arg(category), sqlc.arg(description),
    sqlc.arg(reporter_buyer_id),
    sqlc.arg(now), sqlc.arg(now)
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: DuplicateBuyerReport :one
SELECT *
FROM seller.reports
WHERE reporter_buyer_id = sqlc.arg(buyer_id)::bigint AND store_id = sqlc.arg(store_id) AND category = sqlc.arg(category) AND outcome IS NULL
  AND (created_at AT TIME ZONE 'Asia/Taipei')::date = (sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Taipei')::date;

-- name: InsertReportSupplement :one
INSERT INTO seller.report_supplements (id, report_id, seq, description, created_at)
VALUES (sqlc.arg(id), sqlc.arg(report_id), sqlc.arg(seq), sqlc.arg(description), sqlc.arg(now))
RETURNING *;

-- name: SupplementCount :one
SELECT count(*)::int
FROM seller.report_supplements
WHERE report_id = $1;

-- name: ReportSupplements :many
SELECT *
FROM seller.report_supplements
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[])
ORDER BY report_id, seq;

-- name: InsertReportEvidence :exec
INSERT INTO seller.report_evidence (report_id, submission_seq, position, supplement_id, asset_id, object_key)
VALUES (sqlc.arg(report_id), sqlc.arg(submission_seq), sqlc.arg(position), sqlc.narg(supplement_id), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: ReportEvidence :many
SELECT *
FROM seller.report_evidence
WHERE report_id = $1
ORDER BY submission_seq, position;

-- name: ReportEvidenceOf :many
SELECT *
FROM seller.report_evidence
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[])
ORDER BY report_id, submission_seq, position;

-- name: BumpReportVersion :exec
UPDATE seller.reports
SET version = version + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ReportByID :one
SELECT *
FROM seller.reports
WHERE id = $1;

-- name: LockReport :one
SELECT *
FROM seller.reports
WHERE id = $1
FOR UPDATE;

-- name: LockReports :many
SELECT *
FROM seller.reports
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: TriageReport :one
UPDATE seller.reports r
SET case_id = sqlc.arg(case_id)::uuid,
    assessed_category = sqlc.arg(category)::text,
    triaged_at = sqlc.arg(now)::timestamptz,
    triaged_by_admin_id = sqlc.arg(admin_id)::bigint,
    supplements_seen_seq = coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0),
    version = r.version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE r.id = sqlc.arg(id) AND r.version = sqlc.arg(expected_version)::bigint AND r.case_id IS NULL AND r.outcome IS NULL
RETURNING *;

-- name: MoveReport :one
UPDATE seller.reports r
SET case_id = sqlc.arg(case_id)::uuid,
    assessed_category = sqlc.arg(category)::text,
    supplements_seen_seq = coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0),
    version = r.version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE r.id = sqlc.arg(id) AND r.version = sqlc.arg(expected_version)::bigint AND r.case_id IS NOT NULL AND r.outcome IS NULL
RETURNING *;

-- name: MarkReportSupplementsSeen :one
UPDATE seller.reports r
SET supplements_seen_seq = (SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id)
WHERE r.id = sqlc.arg(id) AND r.outcome IS NULL
  AND EXISTS (SELECT 1 FROM seller.report_supplements p WHERE p.report_id = r.id AND p.seq > r.supplements_seen_seq)
RETURNING *;

-- name: ResolveReport :one
UPDATE seller.reports
SET outcome = sqlc.arg(outcome)::text,
    resolved_at = sqlc.arg(now)::timestamptz,
    resolved_by_admin_id = sqlc.arg(admin_id)::bigint,
    resolution_note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)::bigint AND outcome IS NULL
RETURNING *;

-- name: ResolveCaseReports :many
UPDATE seller.reports
SET outcome = sqlc.arg(outcome)::text,
    resolved_at = sqlc.arg(now)::timestamptz,
    resolved_by_admin_id = sqlc.arg(admin_id)::bigint,
    resolution_note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE case_id = sqlc.arg(case_id) AND outcome IS NULL
RETURNING id, store_id, reporter_buyer_id;

-- name: ReportDetails :many
SELECT sqlc.embed(r),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (SELECT count(*) FROM seller.report_supplements p WHERE p.report_id = r.id)::int AS supplement_count,
       coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0)::int AS latest_supplement_seq,
       (SELECT max(p.created_at) FROM seller.report_supplements p WHERE p.report_id = r.id) AS latest_supplement_at
FROM seller.reports r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE r.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY r.created_at, r.id;

-- name: AdminReports :many
SELECT sqlc.embed(r),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (SELECT count(*) FROM seller.report_supplements p WHERE p.report_id = r.id)::int AS supplement_count,
       coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0)::int AS latest_supplement_seq,
       (SELECT max(p.created_at) FROM seller.report_supplements p WHERE p.report_id = r.id) AS latest_supplement_at
FROM seller.reports r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'AWAITING_TRIAGE' AND r.case_id IS NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'IN_CASE' AND r.case_id IS NOT NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'RESOLVED' AND r.outcome IS NOT NULL))
  AND (sqlc.narg(category)::text IS NULL OR coalesce(r.assessed_category, r.category) = sqlc.narg(category)::text)
  AND (sqlc.narg(store_id)::bigint IS NULL OR r.store_id = sqlc.narg(store_id)::bigint)
  AND (sqlc.narg(case_id)::uuid IS NULL OR r.case_id = sqlc.narg(case_id)::uuid)
  AND (NOT sqlc.arg(citable_as_source)::bool
       OR (s.status <> 'CLOSED' AND (r.outcome IS NULL OR r.outcome NOT IN ('NO_VIOLATION', 'INSUFFICIENT', 'ABUSIVE'))))
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminReportsQueue :many
SELECT sqlc.embed(r),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (SELECT count(*) FROM seller.report_supplements p WHERE p.report_id = r.id)::int AS supplement_count,
       coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0)::int AS latest_supplement_seq,
       (SELECT max(p.created_at) FROM seller.report_supplements p WHERE p.report_id = r.id) AS latest_supplement_at
FROM seller.reports r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'AWAITING_TRIAGE' AND r.case_id IS NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'IN_CASE' AND r.case_id IS NOT NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'RESOLVED' AND r.outcome IS NOT NULL))
  AND (sqlc.narg(category)::text IS NULL OR coalesce(r.assessed_category, r.category) = sqlc.narg(category)::text)
  AND (sqlc.narg(store_id)::bigint IS NULL OR r.store_id = sqlc.narg(store_id)::bigint)
  AND (sqlc.narg(case_id)::uuid IS NULL OR r.case_id = sqlc.narg(case_id)::uuid)
  AND (NOT sqlc.arg(citable_as_source)::bool
       OR (s.status <> 'CLOSED' AND (r.outcome IS NULL OR r.outcome NOT IN ('NO_VIOLATION', 'INSUFFICIENT', 'ABUSIVE'))))
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (r.created_at, r.id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY r.created_at, r.id
LIMIT sqlc.arg(page_size);

-- name: ReportFindingSources :many
SELECT DISTINCT report_id
FROM seller.finding_reports
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[]);

-- name: BuyerReports :many
SELECT *
FROM seller.reports
WHERE reporter_buyer_id = sqlc.arg(buyer_id)::bigint
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: SupplementCounts :many
SELECT report_id, count(*)::int AS supplement_count
FROM seller.report_supplements
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[])
GROUP BY report_id;

-- name: InsertEvidenceAccess :exec
INSERT INTO seller.evidence_accesses (access_id, report_id, submission_seq, request_id, position, admin_id, accessed_at)
VALUES (sqlc.arg(access_id), sqlc.narg(report_id), sqlc.narg(submission_seq), sqlc.narg(request_id), sqlc.arg(position), sqlc.arg(admin_id), sqlc.arg(now));

-- name: ReviewReportTriageByIDs :many
SELECT r.report_id::uuid AS report_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_report_triage(sqlc.arg(at)::timestamptz) r
WHERE r.report_id = ANY(sqlc.arg(ids)::uuid[]);
