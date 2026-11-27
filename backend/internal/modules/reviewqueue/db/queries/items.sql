-- name: ListItems :many
SELECT q.kind, q.item_id, q.report_id, q.case_id, q.inquiry_id, q.authority_order_id, q.target_position, q.account_id,
       q.measure_id, q.request_id, q.kyc_application_id, q.store_id, q.store_name, q.start_at, q.due_at
FROM (
    SELECT s.kind, s.item_id, s.report_id, s.case_id, s.inquiry_id, s.authority_order_id, s.target_position, s.account_id,
           s.measure_id, s.request_id, s.kyc_application_id, s.store_id, s.store_name, s.start_at, s.due_at
    FROM seller.pub_review_items(sqlc.arg(now)::timestamptz) s
    UNION ALL
    SELECT p.kind, p.item_id, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, p.store_id, NULL::text, p.start_at, p.due_at
    FROM payment.pub_review_items(sqlc.arg(now)::timestamptz) p
    UNION ALL
    SELECT m.kind, m.item_id, NULL::uuid, m.case_id, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::bigint, NULL::text, m.start_at, m.due_at
    FROM moderation.pub_review_items(sqlc.arg(now)::timestamptz) m
) q
WHERE q.kind = ANY(sqlc.arg(kinds)::text[])
  AND (sqlc.narg(after_id)::text IS NULL
       OR (q.due_at, q.start_at, q.kind COLLATE "C", q.item_id COLLATE "C")
         > (sqlc.narg(after_due)::timestamptz, sqlc.narg(after_start)::timestamptz,
            sqlc.narg(after_kind)::text COLLATE "C", sqlc.narg(after_id)::text COLLATE "C"))
ORDER BY q.due_at, q.start_at, q.kind COLLATE "C", q.item_id COLLATE "C"
LIMIT sqlc.arg(lim);
