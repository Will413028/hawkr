-- name: InsertViolation :one
INSERT INTO seller.violations (
    id, store_id, account_id, category, severity, escalated_from, origin_measure_id,
    created_by_admin_id, created_at, updated_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(account_id), sqlc.arg(category), sqlc.arg(severity),
    sqlc.narg(escalated_from), sqlc.arg(origin_measure_id), sqlc.arg(admin_id), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: InsertFindingModerationReports :exec
INSERT INTO seller.finding_moderation_reports (violation_id, report_id)
SELECT sqlc.arg(violation_id), unnest(sqlc.arg(report_ids)::uuid[]);

-- name: InsertFindingReports :exec
INSERT INTO seller.finding_reports (violation_id, report_id)
SELECT sqlc.arg(violation_id), unnest(sqlc.arg(report_ids)::uuid[]);

-- name: FindingReportIDs :many
SELECT violation_id, report_id
FROM seller.finding_reports
WHERE violation_id = ANY(sqlc.arg(violation_ids)::uuid[])
ORDER BY violation_id, report_id;

-- name: CountRecentMediumViolations :one
SELECT count(*)
FROM seller.violations
WHERE account_id = sqlc.arg(account_id) AND status = 'ENFORCED' AND COALESCE(escalated_from, severity) = 'MEDIUM'
  AND created_at >= sqlc.arg(since);

-- name: ViolationByID :one
SELECT *
FROM seller.violations
WHERE id = $1;

-- name: ViolationsByIDs :many
SELECT *
FROM seller.violations
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id;

-- name: OriginFindings :many
SELECT *
FROM seller.violations
WHERE origin_measure_id = $1 AND status = 'ENFORCED'
ORDER BY created_at, id;

-- name: RevokeViolations :many
UPDATE seller.violations
SET status = 'REVOKED',
    revoked_reason = sqlc.arg(reason)::text,
    revoked_by_admin_id = sqlc.arg(admin_id)::bigint,
    revoked_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND status = 'ENFORCED'
RETURNING *;

-- name: AdminViolations :many
SELECT *
FROM seller.violations v
WHERE (sqlc.narg(store_id)::bigint IS NULL OR v.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR v.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(status)::text IS NULL OR v.status = sqlc.narg(status))
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (v.created_at, v.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY v.created_at DESC, v.id DESC
LIMIT sqlc.arg(page_size);

-- name: InsertMeasure :one
WITH created AS (
    INSERT INTO seller.measures (
        id, account_id, store_id, kind, basis, risk_ground, lead_finding_id, review_by, statement_to_seller, starts_at, ends_at,
        termination_ground, source_measure_id, source_case_id, source_authority_order_id, current_revision, request_id, request_hash, created_by_admin_id, created_at, updated_at
    )
    VALUES (
        sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(store_id), sqlc.arg(kind), sqlc.arg(basis), sqlc.narg(risk_ground),
        sqlc.narg(lead_finding_id), sqlc.narg(review_by), sqlc.arg(statement_to_seller), sqlc.arg(starts_at), sqlc.narg(ends_at),
        sqlc.narg(termination_ground), sqlc.narg(source_measure_id), sqlc.narg(source_case_id), sqlc.narg(source_authority_order_id),
        1, sqlc.narg(request_id), sqlc.narg(request_hash), sqlc.arg(admin_id), sqlc.arg(now), sqlc.arg(now)
    )
    ON CONFLICT (created_by_admin_id, request_id) WHERE request_id IS NOT NULL DO NOTHING
    RETURNING *
), history AS (
    INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at)
    SELECT id, current_revision, statement_to_seller, ends_at, created_by_admin_id, created_at FROM created
)
SELECT * FROM created;

-- name: InsertMeasureContents :exec
INSERT INTO seller.measure_contents (
    measure_id, content_kind, content_id, snapshot_state, snapshot_body, snapshot_authored_at, snapshot_image_count,
    link_post_id, link_store_id, link_thread_id
)
SELECT sqlc.arg(measure_id)::uuid, (sqlc.arg(kinds)::text[])[i], (sqlc.arg(ids)::uuid[])[i], (sqlc.arg(states)::text[])[i],
       (sqlc.arg(bodies)::text[])[i],
       (sqlc.arg(authored_ats)::timestamptz[])[i],
       (sqlc.arg(image_counts)::int[])[i],
       NULLIF((sqlc.arg(link_post_ids)::uuid[])[i], '00000000-0000-0000-0000-000000000000'::uuid),
       NULLIF((sqlc.arg(link_store_ids)::bigint[])[i], 0::bigint),
       NULLIF((sqlc.arg(link_thread_ids)::uuid[])[i], '00000000-0000-0000-0000-000000000000'::uuid)
FROM generate_series(1, cardinality(sqlc.arg(ids)::uuid[])) AS i;

-- name: ActiveTakedownOfContent :one
SELECT measure_id
FROM seller.measure_contents
WHERE content_kind = sqlc.arg(content_kind) AND content_id = sqlc.arg(content_id) AND measure_status = 'ENFORCED';

-- name: ActiveTakedownOfContents :one
SELECT EXISTS (
    SELECT 1
    FROM seller.measure_contents c
    JOIN generate_series(1, cardinality(sqlc.arg(ids)::uuid[])) AS i
        ON c.content_kind = (sqlc.arg(kinds)::text[])[i] AND c.content_id = (sqlc.arg(ids)::uuid[])[i]
    WHERE c.measure_status = 'ENFORCED'
);

-- name: MeasureContents :many
SELECT measure_id, content_kind, content_id, snapshot_state,
       COALESCE(left(snapshot_body, sqlc.arg(excerpt_runes)::int), '')::text AS snapshot_excerpt,
       COALESCE(char_length(snapshot_body) > sqlc.arg(excerpt_runes)::int, false)::boolean AS snapshot_truncated,
       snapshot_authored_at, snapshot_image_count, link_post_id, link_store_id, link_thread_id
FROM seller.measure_contents
WHERE measure_id = ANY(sqlc.arg(measure_ids)::uuid[])
ORDER BY measure_id, content_kind COLLATE "C", content_id;

-- name: MeasureContentRefs :many
SELECT content_kind, content_id
FROM seller.measure_contents
WHERE measure_id = sqlc.arg(measure_id)
ORDER BY content_kind COLLATE "C", content_id;

-- name: MeasureContentBodies :many
SELECT measure_id, content_kind, content_id, snapshot_body::text AS body
FROM seller.measure_contents
WHERE measure_id = ANY(sqlc.arg(measure_ids)::uuid[]) AND snapshot_state = 'CAPTURED';

-- name: LinkedMeasureExists :one
SELECT EXISTS (
    SELECT 1 FROM seller.measures
    WHERE source_measure_id = sqlc.arg(source_measure_id) AND account_id = sqlc.arg(account_id) AND status = 'ENFORCED'
);

-- name: InsertMeasureFinding :exec
INSERT INTO seller.measure_findings (measure_id, violation_id)
VALUES (sqlc.arg(measure_id), sqlc.arg(violation_id));

-- name: MeasureFindings :many
SELECT mf.measure_id, sqlc.embed(v)
FROM seller.measure_findings mf
JOIN seller.violations v ON v.id = mf.violation_id
JOIN seller.measures m ON m.id = mf.measure_id
WHERE mf.measure_id = ANY(sqlc.arg(measure_ids)::uuid[])
ORDER BY mf.measure_id, (v.id = m.lead_finding_id) DESC, v.created_at, v.id;

-- name: MeasuresCitingFindings :many
SELECT DISTINCT m.*
FROM seller.measure_findings mf
JOIN seller.measures m ON m.id = mf.measure_id
WHERE mf.violation_id = ANY(sqlc.arg(violation_ids)::uuid[]) AND m.status = 'ENFORCED' AND m.id <> sqlc.arg(except_id)
ORDER BY m.id;

-- name: MeasureByID :one
SELECT *
FROM seller.measures
WHERE id = $1;

-- name: LockMeasure :one
SELECT *
FROM seller.measures
WHERE id = $1
FOR UPDATE;

-- name: MeasuresByIDs :many
SELECT *
FROM seller.measures
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id;

-- name: MeasureStoreNames :many
SELECT store_id, store_name
FROM seller.stores
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: MeasureByRequest :one
SELECT *
FROM seller.measures
WHERE created_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: MeasureRevisions :many
SELECT *
FROM seller.measure_revisions
WHERE measure_id = $1
ORDER BY revision;

-- name: AdvanceMeasureRevision :one
WITH advanced AS (
    UPDATE seller.measures
    SET current_revision = current_revision + 1,
        statement_to_seller = sqlc.arg(statement_to_seller)::text,
        ends_at = sqlc.narg(ends_at)::timestamptz,
        updated_at = sqlc.arg(now),
        version = version + 1
    WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED'
    RETURNING *
), history AS (
    INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, system_cause, created_at)
    SELECT id, current_revision, statement_to_seller, ends_at, sqlc.narg(admin_id)::bigint, sqlc.narg(system_cause)::text, updated_at FROM advanced
)
SELECT * FROM advanced;

-- name: TouchMeasure :one
UPDATE seller.measures
SET updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED'
RETURNING *;

-- name: SetMeasureReview :one
UPDATE seller.measures
SET review_by = sqlc.arg(review_by)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED' AND basis = 'RISK'
RETURNING *;

-- name: RevokeMeasure :one
UPDATE seller.measures
SET status = 'REVOKED',
    revoked_reason = sqlc.arg(reason)::text,
    revoked_by_admin_id = sqlc.arg(admin_id)::bigint,
    revoked_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz,
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED'
RETURNING *;

-- name: CascadeRevokeMeasures :many
UPDATE seller.measures
SET status = 'REVOKED',
    revoked_reason = sqlc.arg(reason)::text,
    revoked_by_admin_id = sqlc.arg(admin_id)::bigint,
    revoked_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz,
    version = version + 1
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND status = 'ENFORCED'
RETURNING *;

-- name: InsertGovernanceNote :one
INSERT INTO seller.governance_notes (note_id, measure_id, case_id, admin_id, body, severity_from, severity_to, created_at)
VALUES (sqlc.arg(note_id), sqlc.narg(measure_id), sqlc.narg(case_id), sqlc.arg(admin_id), sqlc.arg(body), sqlc.narg(severity_from), sqlc.narg(severity_to), sqlc.arg(now))
RETURNING *;

-- name: MeasureNotes :many
SELECT *
FROM seller.governance_notes
WHERE measure_id = sqlc.arg(measure_id)
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, note_id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, note_id
LIMIT sqlc.arg(page_size);

-- name: CaseNotes :many
SELECT *
FROM seller.governance_notes
WHERE case_id = sqlc.arg(case_id)
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, note_id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, note_id
LIMIT sqlc.arg(page_size);

-- name: InsertMeasureRequest :one
INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
SELECT sqlc.arg(id)::uuid, m.id, sqlc.arg(kind)::text, 'PENDING', sqlc.arg(reason)::text, sqlc.arg(revision)::integer, sqlc.arg(now)::timestamptz, m.basis
FROM seller.measures m
WHERE m.id = sqlc.arg(measure_id)
RETURNING *;

-- name: InsertRequestEvidence :exec
INSERT INTO seller.measure_request_evidence (request_id, position, asset_id, object_key)
VALUES (sqlc.arg(request_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: RequestEvidence :many
SELECT *
FROM seller.measure_request_evidence
WHERE request_id = $1
ORDER BY position;

-- name: RequestsEvidence :many
SELECT *
FROM seller.measure_request_evidence
WHERE request_id = ANY(sqlc.arg(request_ids)::uuid[])
ORDER BY request_id, position;

-- name: RequestsByMeasures :many
SELECT *
FROM seller.measure_requests
WHERE measure_id = ANY(sqlc.arg(measure_ids)::uuid[])
ORDER BY measure_id, submitted_at DESC, id DESC;

-- name: DecideRequest :one
UPDATE seller.measure_requests
SET state = sqlc.arg(state)::text,
    decision_reason = sqlc.arg(reason)::text,
    decided_by_admin_id = sqlc.arg(admin_id)::bigint,
    decided_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND state = 'PENDING'
RETURNING *;

-- name: CloseRequests :many
UPDATE seller.measure_requests
SET state = 'CLOSED',
    close_cause = 'REVOKED',
    decided_by_admin_id = sqlc.arg(admin_id)::bigint,
    decided_at = sqlc.arg(now)::timestamptz
WHERE measure_id = ANY(sqlc.arg(measure_ids)::uuid[]) AND state = 'PENDING'
RETURNING *;

-- name: CloseEndedRemediations :many
UPDATE seller.measure_requests r
SET state = 'CLOSED',
    close_cause = 'ENDED',
    decided_by_admin_id = NULL,
    decided_at = COALESCE(m.ends_at, r.submitted_at)
FROM seller.measures m
WHERE m.id = r.measure_id AND r.measure_id = ANY(sqlc.arg(measure_ids)::uuid[]) AND r.kind = 'REMEDIATION' AND r.state = 'PENDING'
RETURNING r.*;

-- name: MeasuresWithEndedRemediation :many
SELECT m.*
FROM seller.measures m
WHERE m.status = 'ENFORCED' AND m.kind = 'SUSPENSION' AND m.id <> ALL(sqlc.arg(active_ids)::uuid[])
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND EXISTS (SELECT 1 FROM seller.measure_requests r WHERE r.measure_id = m.id AND r.kind = 'REMEDIATION' AND r.state = 'PENDING')
ORDER BY m.id;


-- name: AccountMeasures :many
SELECT m.*
FROM seller.measures m
WHERE m.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind IN ('WARNING', 'CONTENT_TAKEDOWN'))
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (m.created_at, m.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminMeasures :many
SELECT m.*
FROM seller.measures m
WHERE (sqlc.narg(store_id)::bigint IS NULL OR m.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(source_case_id)::uuid IS NULL OR m.source_case_id = sqlc.narg(source_case_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    JOIN seller.reports r ON r.id = fr.report_id
    WHERE mf.measure_id = m.id AND r.case_id = sqlc.narg(source_case_id)::uuid))
  AND (sqlc.narg(source_report_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fr.report_id = sqlc.narg(source_report_id)::uuid))
  AND (sqlc.narg(source_authority_order_id)::uuid IS NULL OR m.source_authority_order_id = sqlc.narg(source_authority_order_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.narg(source_authority_order_id)::uuid))
  AND (sqlc.narg(basis)::text IS NULL OR m.basis = sqlc.narg(basis))
  AND (sqlc.narg(request_state)::text IS NULL AND sqlc.narg(request_kind)::text IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_requests r
    WHERE r.measure_id = m.id
      AND (sqlc.narg(request_kind)::text IS NULL OR r.kind = sqlc.narg(request_kind)::text)
      AND (sqlc.narg(request_state)::text IS NULL OR sqlc.narg(request_state)::text =
        CASE WHEN r.state = 'PENDING' AND r.kind = 'REMEDIATION' AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) THEN 'CLOSED' ELSE r.state END)))
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind IN ('WARNING', 'CONTENT_TAKEDOWN'))
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (m.created_at, m.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminMeasuresPendingRequests :many
SELECT sqlc.embed(m), q.submitted_at AS sort_at
FROM seller.measures m
JOIN seller.measure_requests q ON q.measure_id = m.id AND q.state = 'PENDING'
  AND (q.kind = 'APPEAL' OR m.id = ANY(sqlc.arg(active_ids)::uuid[]))
WHERE (sqlc.narg(store_id)::bigint IS NULL OR m.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(source_case_id)::uuid IS NULL OR m.source_case_id = sqlc.narg(source_case_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    JOIN seller.reports r ON r.id = fr.report_id
    WHERE mf.measure_id = m.id AND r.case_id = sqlc.narg(source_case_id)::uuid))
  AND (sqlc.narg(source_report_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fr.report_id = sqlc.narg(source_report_id)::uuid))
  AND (sqlc.narg(source_authority_order_id)::uuid IS NULL OR m.source_authority_order_id = sqlc.narg(source_authority_order_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.narg(source_authority_order_id)::uuid))
  AND (sqlc.narg(basis)::text IS NULL OR m.basis = sqlc.narg(basis))
  AND (sqlc.narg(request_kind)::text IS NULL OR q.kind = sqlc.narg(request_kind)::text)
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind IN ('WARNING', 'CONTENT_TAKEDOWN'))
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (q.submitted_at, m.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY q.submitted_at, m.id
LIMIT sqlc.arg(page_size);

-- name: AdminMeasuresReviewDue :many
SELECT sqlc.embed(m), m.review_by AS sort_at
FROM seller.measures m
WHERE m.id IN (SELECT r.measure_id FROM seller.review_measure_review(sqlc.arg(now)::timestamptz) r)
  AND (sqlc.narg(store_id)::bigint IS NULL OR m.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(source_case_id)::uuid IS NULL OR m.source_case_id = sqlc.narg(source_case_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    JOIN seller.reports r ON r.id = fr.report_id
    WHERE mf.measure_id = m.id AND r.case_id = sqlc.narg(source_case_id)::uuid))
  AND (sqlc.narg(source_report_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fr.report_id = sqlc.narg(source_report_id)::uuid))
  AND (sqlc.narg(source_authority_order_id)::uuid IS NULL OR m.source_authority_order_id = sqlc.narg(source_authority_order_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.narg(source_authority_order_id)::uuid))
  AND (sqlc.narg(basis)::text IS NULL OR m.basis = sqlc.narg(basis))
  AND (sqlc.narg(request_state)::text IS NULL AND sqlc.narg(request_kind)::text IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_requests r
    WHERE r.measure_id = m.id
      AND (sqlc.narg(request_kind)::text IS NULL OR r.kind = sqlc.narg(request_kind)::text)
      AND (sqlc.narg(request_state)::text IS NULL OR sqlc.narg(request_state)::text =
        CASE WHEN r.state = 'PENDING' AND r.kind = 'REMEDIATION' AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) THEN 'CLOSED' ELSE r.state END)))
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind IN ('WARNING', 'CONTENT_TAKEDOWN'))
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (m.review_by, m.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY m.review_by, m.id
LIMIT sqlc.arg(page_size);

-- name: LinkedMeasureAccounts :many
SELECT DISTINCT account_id
FROM seller.measures
WHERE source_measure_id = ANY(sqlc.arg(source_ids)::uuid[]) AND status = 'ENFORCED'
ORDER BY account_id;

-- name: LockLinkedMeasures :many
SELECT *
FROM seller.measures
WHERE source_measure_id = ANY(sqlc.arg(source_ids)::uuid[]) AND status = 'ENFORCED'
ORDER BY id
FOR UPDATE;

-- name: LinkedMeasuresOf :many
SELECT id, account_id
FROM seller.measures
WHERE source_measure_id = $1 AND status = 'ENFORCED';

-- name: QualifyingCitedFindings :many
SELECT v.id
FROM seller.violations v
JOIN seller.measures om ON om.id = v.origin_measure_id
WHERE v.id = ANY(sqlc.arg(ids)::uuid[]) AND v.account_id = sqlc.arg(account_id) AND v.status = 'ENFORCED'
  AND om.created_at <= sqlc.arg(older_than)::timestamptz
ORDER BY v.id;

-- name: OpenNoticeTerminations :many
SELECT *
FROM seller.measures
WHERE account_id = $1 AND status = 'ENFORCED' AND kind = 'TERMINATION' AND termination_ground = 'NOTICE' AND ends_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: ReviewMeasureAppealByIDs :many
SELECT r.request_id::uuid AS request_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_measure_appeal(sqlc.arg(at)::timestamptz) r
WHERE r.request_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ReviewMeasureLiftRequestByIDs :many
SELECT r.request_id::uuid AS request_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_measure_lift_request(sqlc.arg(at)::timestamptz) r
WHERE r.request_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ReviewMeasureReviewByIDs :many
SELECT r.measure_id::uuid AS measure_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_measure_review(sqlc.arg(at)::timestamptz) r
WHERE r.measure_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ClearExpiredContentSnapshots :execrows
WITH due AS (
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measure_contents c
    JOIN seller.measures m ON m.id = c.measure_id
    WHERE c.snapshot_state = 'CAPTURED' AND c.measure_status = 'REVOKED' AND m.revoked_at <= sqlc.arg(cutoff)::timestamptz
    ORDER BY c.measure_id, c.content_kind, c.content_id
    LIMIT sqlc.arg(max_rows)::int
    FOR UPDATE OF c SKIP LOCKED
)
UPDATE seller.measure_contents c
SET snapshot_state = 'CLEARED', snapshot_body = NULL, snapshot_authored_at = NULL, snapshot_image_count = NULL,
    link_post_id = NULL, link_store_id = NULL, link_thread_id = NULL
FROM due
WHERE c.measure_id = due.measure_id AND c.content_kind = due.content_kind AND c.content_id = due.content_id;

-- name: ClearContentSnapshotsOfStores :execrows
UPDATE seller.measure_contents c
SET snapshot_state = 'CLEARED', snapshot_body = NULL, snapshot_authored_at = NULL, snapshot_image_count = NULL,
    link_post_id = NULL, link_store_id = NULL, link_thread_id = NULL
FROM seller.measures m
WHERE m.id = c.measure_id AND m.store_id = ANY(sqlc.arg(store_ids)::bigint[]) AND c.snapshot_state = 'CAPTURED';
