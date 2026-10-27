-- B-169 Food Safety Reports queries
-- 公開無登入食安通報。silent drop 由 schema 上的 phone+seller+ISO week partial unique 強制（B1）；
-- 重複時 INSERT 觸發 unique violation，application 捕捉並回 既有 report_id（仍回 200 OK）。

-- name: CreateFoodSafetyReport :one
INSERT INTO food_safety_reports (
  id,
  target_seller_id,
  target_product_id,
  target_order_no,
  reporter_phone,
  reporter_email,
  reporter_name,
  description,
  evidence_asset_ids,
  status,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
) RETURNING *;

-- name: GetFoodSafetyReportByID :one
SELECT * FROM food_safety_reports WHERE id = $1;

-- name: GetExistingFoodSafetyByPhoneAndSellerThisWeek :one
-- 配合 B1 partial unique：INSERT 觸發 unique violation 後 application 抓既有 report_id 回 client（silent drop 路徑）。
-- 條件需與 unique index 完全對齊（phone + target_seller + UTC ISO week）。
SELECT * FROM food_safety_reports
WHERE reporter_phone = $1
  AND target_seller_id = $2
  AND date_trunc('week', (created_at  AT TIME ZONE 'UTC'))
    = date_trunc('week', (NOW()       AT TIME ZONE 'UTC'))
LIMIT 1;

-- name: AdminListFoodSafetyReports :many
SELECT * FROM food_safety_reports
WHERE
  (sqlc.narg('status')::text   IS NULL OR status   = sqlc.narg('status')::text)
  AND (sqlc.narg('since')::timestamptz IS NULL OR created_at >= sqlc.narg('since')::timestamptz)
  AND (sqlc.narg('unacked_only')::boolean IS NULL
       OR sqlc.narg('unacked_only')::boolean = false
       OR acknowledged_at IS NULL)
ORDER BY
  -- 未 ack 優先（admin UI 視覺強調區）
  (acknowledged_at IS NOT NULL),
  created_at ASC
LIMIT $1 OFFSET $2;

-- name: AdminGetFoodSafetyReport :one
SELECT * FROM food_safety_reports WHERE id = $1;

-- name: AdminUpdateFoodSafetyReportStatus :one
-- admin PATCH /admin/food_safety_reports/{id}
UPDATE food_safety_reports SET
  status                  = $2,
  investigating_at        = $3,
  resolved_at             = $4,
  admin_resolution_summary = $5,
  updated_at              = NOW()
WHERE id = $1
RETURNING *;

-- name: AcknowledgeFoodSafetyReport :one
-- POST /admin/food_safety_reports/{id}/ack
-- only-if 條件 `acknowledged_at IS NULL` 確保 idempotent：重複 ack 不覆寫既有時間戳；
-- application 層比對 RowsAffected 判斷是否為重複呼叫，回 409 並回傳既有 ack 資訊。
UPDATE food_safety_reports SET
  acknowledged_at         = NOW(),
  acknowledged_by_admin_id = $2,
  updated_at              = NOW()
WHERE id = $1
  AND acknowledged_at IS NULL
RETURNING *;

-- name: MarkFoodSafetyFDAReported :one
-- POST /admin/food_safety_reports/{id}/mark_fda_reported （D-Q3）
-- 已 mark 過則 RowsAffected=0，application 回 409 + 既有資料。
UPDATE food_safety_reports SET
  fda_reported_at    = NOW(),
  fda_report_channel = $2,
  fda_reference_no   = $3,
  updated_at         = NOW()
WHERE id = $1
  AND fda_reported_at IS NULL
RETURNING *;

-- name: AnonymizeOldFoodSafetyPhones :many
-- task 9.1 cron worker：超過 90 天的 record，phone / email 替換為 hash 形式，OSS 實體刪除由 worker 另行處理。
-- 條件：尚未被 anonymize 過（phone 不以 phone_hash: 開頭）。
UPDATE food_safety_reports SET
  reporter_phone = 'phone_hash:' || encode(sha256(reporter_phone::bytea), 'hex'),
  reporter_email = CASE
    WHEN reporter_email IS NULL THEN NULL
    ELSE 'email_hash:' || encode(sha256(reporter_email::bytea), 'hex')
  END,
  updated_at = NOW()
WHERE created_at < NOW() - INTERVAL '90 days'
  AND reporter_phone NOT LIKE 'phone_hash:%'
RETURNING id, evidence_asset_ids;
