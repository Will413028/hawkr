-- B-169 Seller Violations queries
-- admin 建立的違規處罰紀錄。狀態兩值（D11 enforced / revoked），「目前是否生效」由
-- (revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())) 即時計算。

-- ========== Lock & Auto-Escalation ==========

-- name: AcquireViolationCreateLock :exec
-- D6: 同 seller 並發建立 violation 串行。advisory lock 會在 transaction commit / rollback 時自動釋放。
-- caller 必須處於 transaction 內，否則 lock 立即釋放等於沒鎖。
-- 注意：pg_advisory_xact_lock 只有 (bigint) 與 (int, int) 兩種簽章，無 (int, bigint)。
-- seller_id 是 TSID（64-bit，可能超出 int4 範圍），不可硬塞進兩參數版；
-- 改用 hashtextextended(text, bigint) 把 'violation_create' 命名空間與 seller_id 一起雜湊成單一 bigint key，
-- 對應單參數版 pg_advisory_xact_lock(bigint)。
SELECT pg_advisory_xact_lock(hashtextextended('violation_create', $1::bigint));

-- name: CountMediumLast90dBySeller :one
-- D6: AcquireViolationCreateLock 後在同 transaction 呼叫；用於 inline auto-escalation 判定。
-- 條件：90 天內 severity='medium' 且未 revoked 的 violation 數量。
SELECT COUNT(*) FROM seller_violations
WHERE seller_id = $1
  AND severity = 'medium'
  AND revoked_at IS NULL
  AND effective_at >= NOW() - INTERVAL '90 days';

-- ========== Create / Mutate ==========

-- name: CreateSellerViolation :one
-- penalty 欄位互斥由 schema chk_sv_penalty_fields enforce；application 層應先驗一次回 4xx 友善訊息。
INSERT INTO seller_violations (
  id,
  seller_id,
  source_seller_report_id,
  source_food_safety_report_id,
  category,
  severity,
  penalty_type,
  penalty_amount_cents,
  description,
  public_summary,
  effective_at,
  expires_at,
  status,
  created_by_admin_id,
  created_at,
  updated_at
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
) RETURNING *;

-- name: GetSellerViolationByID :one
SELECT * FROM seller_violations WHERE id = $1;

-- name: AdminUpdateSellerViolationMutable :one
-- 編輯：只能改 description / public_summary / expires_at（不可改 seller / severity / penalty_type / penalty_amount_cents）。
UPDATE seller_violations SET
  description    = $2,
  public_summary = $3,
  expires_at     = $4,
  updated_at     = NOW()
WHERE id = $1
  AND revoked_at IS NULL
RETURNING *;

-- name: AdminRevokeSellerViolation :one
-- admin 主動 revoke。也可由申訴 accept 路徑 trigger（appeal_status='accepted'）。
UPDATE seller_violations SET
  status         = 'revoked',
  revoked_at     = NOW(),
  revoked_reason = $2,
  updated_at     = NOW()
WHERE id = $1
  AND revoked_at IS NULL
RETURNING *;

-- ========== Listing ==========

-- name: ListSellerViolationsBySellerSelf :many
-- 賣家自助：GET /app/sellers/me/violations，含全 severity / 含 revoked / 不限期間。
SELECT * FROM seller_violations
WHERE seller_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSellerViolationsPublicLast90dHigh :many
-- D8 公開賣家頁：severity high+ AND revoked_at IS NULL AND effective_at >= NOW() - 90d
--               AND (expires_at IS NULL OR expires_at > NOW()) AND public_summary IS NOT NULL
-- public_summary 為 NULL 時不公開（admin 必須撰寫公開摘要）。
SELECT
  id,
  seller_id,
  category,
  severity,
  penalty_type,
  public_summary,
  effective_at,
  expires_at
FROM seller_violations
WHERE seller_id = $1
  AND severity IN ('high', 'critical')
  AND revoked_at IS NULL
  AND effective_at >= NOW() - INTERVAL '90 days'
  AND (expires_at IS NULL OR expires_at > NOW())
  AND public_summary IS NOT NULL
ORDER BY effective_at DESC;

-- name: AdminListSellerViolations :many
SELECT * FROM seller_violations
WHERE
  (sqlc.narg('seller_id')::bigint IS NULL OR seller_id = sqlc.narg('seller_id')::bigint)
  AND (sqlc.narg('severity')::text  IS NULL OR severity  = sqlc.narg('severity')::text)
  AND (sqlc.narg('penalty_type')::text IS NULL OR penalty_type = sqlc.narg('penalty_type')::text)
  AND (sqlc.narg('appeal_status')::text IS NULL OR appeal_status = sqlc.narg('appeal_status')::text)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ========== Cron Worker ==========

-- name: ListExpiredFullSuspensions :many
-- task 9.2 cron：找已過期但 sellers.status 仍 suspended_governance 的 violation。
-- 同 seller 多筆 full_suspension 取最大 expires_at 為基準，由 application 層判斷後再 update。
SELECT * FROM seller_violations
WHERE penalty_type = 'full_suspension'
  AND revoked_at IS NULL
  AND expires_at <= NOW()
ORDER BY seller_id, expires_at DESC;

-- ========== Status recompute (32 §13.3) ==========

-- name: HasActiveFullSuspensionBySeller :one
-- recomputeSellerStatus 讀端：sellerID 為 store surrogate（②）。生效中定義同 §6.2 cron。
SELECT EXISTS(
  SELECT 1 FROM seller_violations
  WHERE seller_id = $1
    AND penalty_type = 'full_suspension'
    AND revoked_at IS NULL
    AND expires_at > NOW()
);

-- ========== Appeal ==========

-- name: SubmitAppeal :one
-- 賣家提交申訴；限申訴一次 → 由 only-if condition `appealed = false` enforce。
-- 重複呼叫 RowsAffected=0，application 回 409。
UPDATE seller_violations SET
  appealed                  = true,
  appeal_reason             = $2,
  appeal_evidence_asset_ids = $3,
  appeal_status             = 'pending',
  appeal_submitted_at       = NOW(),
  updated_at                = NOW()
WHERE id = $1
  AND appealed = false
  AND status = 'enforced'   -- revoked 不可申訴
RETURNING *;

-- name: DecideAppealAccept :one
-- admin POST /admin/violations/{id}/appeal/decide decision='accept'
-- accept 時：appeal_status='accepted' + 自動 revoke + 設 revoked_reason。
UPDATE seller_violations SET
  appeal_status            = 'accepted',
  appeal_decided_at        = NOW(),
  appeal_decided_by_admin_id = $2,
  appeal_decision_reason   = $3,
  status                   = 'revoked',
  revoked_at               = NOW(),
  revoked_reason           = 'appeal_accepted',
  updated_at               = NOW()
WHERE id = $1
  AND appealed = true
  AND appeal_status = 'pending'
RETURNING *;

-- name: DecideAppealReject :one
-- decision='reject' 時：appeal_status='rejected'，violation status 不變。
UPDATE seller_violations SET
  appeal_status            = 'rejected',
  appeal_decided_at        = NOW(),
  appeal_decided_by_admin_id = $2,
  appeal_decision_reason   = $3,
  updated_at               = NOW()
WHERE id = $1
  AND appealed = true
  AND appeal_status = 'pending'
RETURNING *;
