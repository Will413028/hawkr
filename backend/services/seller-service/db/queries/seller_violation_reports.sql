-- B-169 Seller Violation Reports queries
-- 登入端商家違規檢舉。dedup 由 schema 上的 calendar-day partial unique 強制（D13）；
-- 重複時 INSERT 觸發 unique violation，application 捕捉並回 既有 report_id。

-- name: CreateSellerViolationReport :one
INSERT INTO seller_violation_reports (
  id,
  target_seller_id,
  target_product_id,
  target_post_id,
  target_order_no,
  reporter_account_id,
  category,
  severity,
  description,
  evidence_asset_ids,
  status,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: GetSellerViolationReportByID :one
SELECT * FROM seller_violation_reports WHERE id = $1;

-- name: GetExistingReportByReporterAndTargetToday :one
-- 配合 D13 partial unique：INSERT 觸發 unique violation 後，application 用此 query 抓既有 report_id 回 client。
-- 條件需與 unique index 完全對齊（reporter + target seller + category + UTC calendar day）。
SELECT * FROM seller_violation_reports
WHERE reporter_account_id = $1
  AND target_seller_id = $2
  AND category = $3
  AND ((created_at AT TIME ZONE 'UTC')::date) = ((NOW() AT TIME ZONE 'UTC')::date)
LIMIT 1;

-- name: ListSellerViolationReportsByReporter :many
SELECT * FROM seller_violation_reports
WHERE reporter_account_id = $1
  AND created_at >= $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: AdminListSellerViolationReports :many
-- admin 端 filter list（status / severity / since）；用於 GET /admin/violation_reports 中 source=seller_violation_reports 的部分。
SELECT * FROM seller_violation_reports
WHERE
  (sqlc.narg('status')::text   IS NULL OR status   = sqlc.narg('status')::text)
  AND (sqlc.narg('severity')::text IS NULL OR severity = sqlc.narg('severity')::text)
  AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since')::timestamptz)
ORDER BY
  CASE severity
    WHEN 'critical' THEN 0
    WHEN 'high'     THEN 1
    WHEN 'medium'   THEN 2
    WHEN 'low'      THEN 3
    ELSE 4
  END,
  created_at ASC
LIMIT $1 OFFSET $2;

-- name: AdminGetSellerViolationReport :one
SELECT * FROM seller_violation_reports WHERE id = $1;

-- name: AdminUpdateSellerViolationReportStatus :one
-- admin PATCH /admin/seller_violation_reports/{id} 推進 status（pending → investigating → verified / dismissed）。
-- 狀態合法性由 application 層檢查；此處純 update。
UPDATE seller_violation_reports SET
  status                  = $2,
  investigating_at        = $3,
  resolved_at             = $4,
  admin_resolution_summary = $5,
  updated_at              = NOW()
WHERE id = $1
RETURNING *;
