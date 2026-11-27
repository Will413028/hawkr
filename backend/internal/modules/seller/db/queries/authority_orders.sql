-- name: InsertAuthorityOrder :one
INSERT INTO seller.authority_orders (
    id, agency, document_no, received_at, kind, summary, due_at, hold_until, registered_at, registered_by_admin_id, request_id, request_hash)
VALUES (
    sqlc.arg(id), sqlc.arg(agency), sqlc.arg(document_no), sqlc.arg(received_at), sqlc.arg(kind), sqlc.arg(summary), sqlc.arg(due_at),
    sqlc.arg(hold_until), sqlc.arg(now), sqlc.arg(admin_id), sqlc.narg(request_id), sqlc.narg(request_hash))
ON CONFLICT (registered_by_admin_id, request_id) DO NOTHING
RETURNING *;

-- name: AuthorityOrderByRequest :one
SELECT *
FROM seller.authority_orders
WHERE registered_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: InsertAuthorityOrderTarget :exec
INSERT INTO seller.authority_order_targets (order_id, position, account_id, store_id, product_ids)
VALUES (sqlc.arg(order_id), sqlc.arg(position), sqlc.arg(account_id), sqlc.narg(store_id), sqlc.arg(product_ids)::text[]);

-- name: InsertAuthorityOrderPrepurgedStores :exec
INSERT INTO seller.authority_order_prepurged_stores (order_id, store_id)
SELECT DISTINCT t.order_id, s.store_id
FROM seller.authority_order_targets t
JOIN seller.stores s ON s.account_id = t.account_id AND (t.store_id IS NULL OR s.store_id = t.store_id)
WHERE t.order_id = $1 AND s.pii_purged_at IS NOT NULL;

-- name: AuthorityOrderByID :one
SELECT *
FROM seller.authority_orders
WHERE id = $1;

-- name: LockAuthorityOrder :one
SELECT *
FROM seller.authority_orders
WHERE id = $1
FOR UPDATE;

-- name: AuthorityOrderTargets :many
SELECT t.order_id, t.position, t.account_id, t.store_id, t.product_ids, t.notice, t.notified_at, t.notice_note, t.recheck_on,
       t.notice_recorded_at, t.notice_recorded_by_admin_id, t.prev_position
FROM seller.authority_order_targets t
WHERE t.order_id = ANY(sqlc.arg(order_ids)::uuid[])
ORDER BY t.order_id, t.position;

-- name: AuthorityTargetStores :many
SELECT t.order_id, t.position, s.store_id, s.store_name, s.pii_purged_at, (p.store_id IS NOT NULL)::boolean AS purged_at_registration,
       (s.status = 'CLOSED')::boolean AS closed,
       (s.store_id = (SELECT max(l.store_id) FROM seller.stores l WHERE l.account_id = s.account_id))::boolean AS latest_of_account
FROM seller.authority_order_targets t
JOIN seller.stores s ON s.account_id = t.account_id AND (t.store_id IS NULL OR s.store_id = t.store_id)
LEFT JOIN seller.authority_order_prepurged_stores p ON p.order_id = t.order_id AND p.store_id = s.store_id
WHERE t.order_id = ANY(sqlc.arg(order_ids)::uuid[])
ORDER BY t.order_id, t.position, s.store_id;

-- name: AdminAuthorityOrders :many
SELECT o.*
FROM seller.authority_orders o
WHERE (sqlc.narg(state)::text IS NULL
        OR (sqlc.narg(state)::text = 'OPEN' AND o.completed_at IS NULL AND o.revoked_at IS NULL)
        OR (sqlc.narg(state)::text = 'COMPLETED' AND o.completed_at IS NOT NULL)
        OR (sqlc.narg(state)::text = 'REVOKED' AND o.revoked_at IS NOT NULL))
  AND (sqlc.narg(account_id)::bigint IS NULL OR EXISTS (
    SELECT 1 FROM seller.authority_order_targets t WHERE t.order_id = o.id AND t.account_id = sqlc.narg(account_id)::bigint))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL
    OR (sqlc.narg(state)::text = 'OPEN' AND (o.due_at, o.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid))
    OR (sqlc.narg(state)::text IS DISTINCT FROM 'OPEN' AND (o.received_at, o.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)))
ORDER BY
    CASE WHEN sqlc.narg(state)::text = 'OPEN' THEN o.due_at END ASC,
    CASE WHEN sqlc.narg(state)::text = 'OPEN' THEN o.id END ASC,
    o.received_at DESC, o.id DESC
LIMIT sqlc.arg(page_size);

-- name: CompleteAuthorityOrder :one
UPDATE seller.authority_orders
SET completed_at = sqlc.arg(now)::timestamptz, completed_by_admin_id = sqlc.arg(admin_id)::bigint, resolution_summary = sqlc.arg(resolution_summary)::text,
    hold_until = sqlc.arg(hold_until)::timestamptz, version = version + 1
WHERE id = sqlc.arg(id) AND completed_at IS NULL AND revoked_at IS NULL AND version = sqlc.arg(expected_version) AND hold_until <= sqlc.arg(hold_until)::timestamptz
RETURNING *;

-- name: RevokeAuthorityOrder :one
UPDATE seller.authority_orders
SET revoked_at = sqlc.arg(now)::timestamptz, revoked_by_admin_id = sqlc.arg(admin_id)::bigint, revoke_reason = sqlc.arg(reason)::text, version = version + 1
WHERE id = sqlc.arg(id) AND completed_at IS NULL AND revoked_at IS NULL AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: ExtendAuthorityOrderHold :one
UPDATE seller.authority_orders
SET hold_until = sqlc.arg(hold_until), version = version + 1
WHERE id = sqlc.arg(id) AND revoked_at IS NULL AND version = sqlc.arg(expected_version) AND hold_until < sqlc.arg(hold_until)::timestamptz
RETURNING *;

-- name: BumpAuthorityOrder :one
UPDATE seller.authority_orders
SET version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)
RETURNING *;

-- name: InsertAuthorityHoldExtension :exec
INSERT INTO seller.authority_order_hold_extensions (order_id, seq, from_until, to_until, cause, reason, admin_id, created_at)
SELECT sqlc.arg(order_id)::uuid, COALESCE(max(e.seq), 0) + 1, sqlc.arg(from_until)::timestamptz, sqlc.arg(to_until)::timestamptz,
       sqlc.arg(cause)::text, sqlc.narg(reason)::text, sqlc.arg(admin_id)::bigint, sqlc.arg(now)::timestamptz
FROM seller.authority_order_hold_extensions e
WHERE e.order_id = sqlc.arg(order_id)::uuid;

-- name: AuthorityHoldExtensions :many
SELECT order_id, seq, from_until, to_until, cause, reason, admin_id, created_at
FROM seller.authority_order_hold_extensions
WHERE order_id = ANY(sqlc.arg(order_ids)::uuid[])
ORDER BY order_id, seq;

-- name: SetAuthorityTargetNotice :execrows
UPDATE seller.authority_order_targets
SET notice = sqlc.arg(notice)::text, notified_at = sqlc.narg(notified_at), notice_note = sqlc.narg(notice_note), recheck_on = sqlc.narg(recheck_on),
    notice_recorded_at = sqlc.arg(now), notice_recorded_by_admin_id = sqlc.arg(admin_id)
WHERE order_id = sqlc.arg(order_id) AND position = sqlc.arg(position) AND notice IS NULL;

-- name: ReplaceWithheldAuthorityTargetNotice :execrows
UPDATE seller.authority_order_targets
SET notice = sqlc.arg(notice)::text, notified_at = sqlc.narg(notified_at), notice_note = sqlc.narg(notice_note), recheck_on = sqlc.narg(recheck_on),
    notice_recorded_at = sqlc.arg(now), notice_recorded_by_admin_id = sqlc.arg(admin_id)
WHERE order_id = sqlc.arg(order_id) AND position = sqlc.arg(position) AND notice = 'WITHHELD';

-- name: InsertAuthorityTargetNoticeHistory :exec
INSERT INTO seller.authority_target_notice_history (
    order_id, position, seq, notice, notice_note, recheck_on, recorded_at, recorded_by_admin_id, replaced_at, replaced_by_admin_id)
SELECT t.order_id, t.position,
       (SELECT COALESCE(max(h.seq), 0) + 1 FROM seller.authority_target_notice_history h WHERE h.order_id = t.order_id AND h.position = t.position),
       t.notice, t.notice_note, t.recheck_on, t.notice_recorded_at, t.notice_recorded_by_admin_id, sqlc.arg(now)::timestamptz, sqlc.arg(admin_id)::bigint
FROM seller.authority_order_targets t
WHERE t.order_id = sqlc.arg(order_id) AND t.position = sqlc.arg(position) AND t.notice = 'WITHHELD';

-- name: AuthorityTargetNoticeHistory :many
SELECT order_id, position, seq, notice, notice_note, recheck_on, recorded_at, recorded_by_admin_id, replaced_at, replaced_by_admin_id
FROM seller.authority_target_notice_history
WHERE order_id = sqlc.arg(order_id)
ORDER BY position, seq;

-- name: OrdersNamingMeasureTarget :many
SELECT o.id, (o.revoked_at IS NOT NULL)::boolean AS revoked
FROM seller.authority_orders o
WHERE o.id = ANY(sqlc.arg(order_ids)::uuid[])
  AND EXISTS (
    SELECT 1 FROM seller.authority_order_targets t
    WHERE t.order_id = o.id AND t.account_id = sqlc.arg(account_id) AND (t.store_id IS NULL OR t.store_id = sqlc.arg(store_id)))
ORDER BY o.id
FOR SHARE OF o;

-- name: AuthorityOrderMeasureTargets :many
SELECT DISTINCT m.account_id, m.store_id
FROM seller.measures m
WHERE m.source_authority_order_id = sqlc.arg(order_id) OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.arg(order_id));

-- name: InsertFindingAuthorityOrders :exec
INSERT INTO seller.finding_authority_orders (violation_id, order_id)
SELECT sqlc.arg(violation_id)::uuid, unnest(sqlc.arg(order_ids)::uuid[]);

-- name: ReviewAuthorityOrderByIDs :many
SELECT r.authority_order_id::uuid AS authority_order_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_authority_order(sqlc.arg(at)::timestamptz) r
WHERE r.authority_order_id = ANY(sqlc.arg(ids)::uuid[]);
