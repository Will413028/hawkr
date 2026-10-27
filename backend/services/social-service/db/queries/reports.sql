-- name: GetReportByReporterAndTarget :one
-- 24h idempotency check: returns most recent report by reporter for the same target
SELECT * FROM reports
WHERE reporter_id = $1
  AND target_type = $2
  AND target_id = $3
  AND create_time > NOW() - INTERVAL '24 hours'
ORDER BY create_time DESC
LIMIT 1;

-- name: CreateReport :one
INSERT INTO reports (
    id, reporter_id, target_type, target_id, target_owner_id, reason_code, description, evidence, target_snapshot
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListReportsByReporter :many
SELECT * FROM reports
WHERE reporter_id = $1
ORDER BY create_time DESC
LIMIT $2 OFFSET $3;

-- name: AdminListReports :many
SELECT * FROM reports
WHERE ($1::text = '' OR status = $1)
  AND ($2::text = '' OR target_type = $2)
  AND ($3::text = '' OR reason_code = $3)
ORDER BY create_time DESC
LIMIT $4 OFFSET $5;

-- name: AdminGetReport :one
SELECT * FROM reports WHERE id = $1 LIMIT 1;

-- name: AdminResolveReport :one
-- 可裁決的起始狀態由呼叫端帶入（PENDING = 還沒人看過、APPEALED = 處置過但被申訴
-- 重開），SQL 內不寫字面值。這道述詞同時是 no-op 保護：命中 0 列即代表單子已經是
-- 終態，usecase 據此回 ErrAlreadyResolved。
UPDATE reports SET
    status          = $2,
    resolved_action = $3,
    resolved_by     = $4,
    resolved_at     = NOW(),
    update_time     = NOW()
WHERE id = $1 AND status = ANY(sqlc.arg(resolvable_statuses)::text[])
RETURNING *;

-- ============================================================================
-- Legacy compatibility: post-only report API used by existing buyer flow
-- (services/social-service/internal/usecase/post.go ReportPost).
-- These wrap the unified `reports` table with target_type='POST'.
-- ============================================================================

-- name: GetPostReport :one
-- 碼值由呼叫端帶入（codemeta.ReportTargetTypePost），SQL 內不寫字面值。
SELECT * FROM reports
WHERE target_type = sqlc.arg(target_type)
  AND target_id   = $1
  AND reporter_id = $2
ORDER BY create_time DESC
LIMIT 1;

-- name: CreatePostReport :one
-- 同上：target_type 由呼叫端帶入。
INSERT INTO reports (id, reporter_id, target_type, target_id, target_owner_id, reason_code, description)
VALUES ($1, $2, sqlc.arg(target_type), $3, $4, $5, $6)
RETURNING *;

-- name: CountReportsByReporterSince :one
SELECT COUNT(*) FROM reports
WHERE reporter_id = $1 AND create_time > $2;

-- name: ReopenReport :one
-- 目標狀態與「可被重開的終態」清單都由呼叫端帶入。終態清單是 ReportStatus 扣掉
-- PENDING／APPEALED 的補集：重開一張還沒人看過、或已經在申訴中的單子沒有意義，
-- 而讓它通過會把 update_time 洗掉卻不改變任何狀態。
UPDATE reports SET
    status      = sqlc.arg(appealed_status),
    update_time = NOW()
WHERE id = $1 AND status = ANY(sqlc.arg(reopenable_statuses)::text[])
RETURNING *;

-- name: CountRecentActionsByOwner :one
-- 動作清單由呼叫端帶入。⚠️ 這是「算不算前科」的業務述詞，不是 resolved_action 的
-- 值域：它是 ReportResolveAction 的**真子集**，刻意排除 DISMISS——駁回代表檢舉不
-- 成立，把它計入累犯次數會讓被誣告的人在管理員眼裡變成慣犯。收斂碼值時不要圖省事
-- 換成整包 AllReportResolveActions。
SELECT COUNT(*) FROM reports
WHERE target_owner_id = $1
  AND resolved_action = ANY(sqlc.arg(prior_offence_actions)::text[])
  AND resolved_at > NOW() - INTERVAL '90 days';
