-- name: ListItems :many
-- 後台待辦：各來源發布的 pub_*_review_items 依期限由急到緩（step10e-3d-design.md §2.2）。現在是 seller、payment（收款帳戶，收款開通 ADR D28）與 moderation（進行中的檢舉案件）。
-- moderation 的列只有案件 ID（case_id 欄）。payment 的列只有店家 ID，店名在這裡是 NULL，由後台的入口組裝（httpapi/reviewqueue）。
-- 全序是 (due_at, start_at, kind, item_id)，kind 與 item_id 以位元組順序（COLLATE "C"）比，與資料庫的預設 collation 無關；
-- keyset 用同樣的 row-value 比較；第一頁沒有游標（四個 after_* 都是 NULL），照 repo 的慣例寫 IS NULL OR。
-- 整句是一個語句快照（READ COMMITTED 下每個語句一個快照），不需要 transaction。
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
