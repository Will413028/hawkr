-- 代行授權列與它的治理歷史。

-- name: GetDelegationAuthorizationForStore :one
-- 代行判定用（preflight、建立 session）：店家層級的列存在就用它（不論有沒有效），沒有才退回全域列。有效性由
-- 呼叫方判斷：寫進 WHERE 的話，一列已撤回的店家授權會讓查詢落到全域列上，逐店的撤回就被推翻了。
SELECT * FROM delegation.seller_delegation_authorizations
WHERE target_store_id = sqlc.arg(target_store_id)::bigint OR target_store_id IS NULL
ORDER BY (target_store_id IS NULL)
LIMIT 1;

-- name: GetDelegationAuthorizationOfStore :one
-- 治理用的單店詳情：只看店家層級的那一列，不退回全域列（治理畫面不能把全域列當成這家店的設定）。管理員的顯示名
-- 是 email（admins 沒有名稱欄），讀取時解析（跟著改名）；撤回者沒有時是空字串。
SELECT * FROM delegation.seller_delegation_authorizations
WHERE target_store_id = sqlc.arg(target_store_id)::bigint;

-- name: LockDelegationAuthorizationOfStore :one
-- 寫入前鎖住變更前的狀態：只為了讓治理歷史的 before 是真的變更前，不參與判定（判定一律看 CAS 的列數）。
-- 列不存在時不鎖：兩個同時的首建都會走到 INSERT，一個成功、另一個的 DO UPDATE 述詞落空。
SELECT * FROM delegation.seller_delegation_authorizations
WHERE target_store_id = sqlc.arg(target_store_id)::bigint
FOR UPDATE;

-- name: ListDelegationAuthorizations :many
-- 治理總覽，新到舊，排除全域列。status 篩選寫成布林相等，effective 與 ineffective 在語法上就是互補的；有效的
-- 定義與 DelegationAuthorization.EffectiveAt 相同（沒有撤回、已經生效、還沒到期）。
SELECT a.*
FROM delegation.seller_delegation_authorizations a
WHERE a.target_store_id IS NOT NULL
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR a.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(status)::text = ''
       OR (sqlc.arg(status)::text = 'effective')
          = (a.revoked_at IS NULL
             AND a.effective_from <= sqlc.arg(now)::timestamptz
             AND (a.effective_until IS NULL OR a.effective_until > sqlc.arg(now)::timestamptz)))
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationAuthorizations :one
-- 與 ListDelegationAuthorizations 相同的篩選。
SELECT count(*) FROM delegation.seller_delegation_authorizations a
WHERE a.target_store_id IS NOT NULL
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR a.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(status)::text = ''
       OR (sqlc.arg(status)::text = 'effective')
          = (a.revoked_at IS NULL
             AND a.effective_from <= sqlc.arg(now)::timestamptz
             AND (a.effective_until IS NULL OR a.effective_until > sqlc.arg(now)::timestamptz)));

-- name: GrantDelegationAuthorization :one
-- 首建與重新開通共用的單句 CAS。撞到有效的列時沒有列回傳（9285 或冪等重放，由呼叫方判斷），不是錯誤；不能改成
-- 先查再決定 INSERT 或 UPDATE（兩個管理員同時開通時一個會靜默覆蓋另一個），也不能接唯一鍵的錯誤（transaction
-- 會中止）。重新開通是一次全新的授權：生效時間、核准者一起重設，撤回的三欄清空。回傳的 id 等於送入的 id 就是
-- 首建（每店終生一列，重新開通時 id 不變）。
INSERT INTO delegation.seller_delegation_authorizations (
    id, target_store_id, source, source_reference, granted_capabilities,
    effective_from, effective_until, approved_by_admin_id, approved_at, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(target_store_id)::bigint, sqlc.arg(source), sqlc.arg(source_reference),
    sqlc.arg(granted_capabilities)::text[],
    sqlc.arg(now)::timestamptz, sqlc.narg(effective_until), sqlc.arg(approved_by_admin_id),
    sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz
)
ON CONFLICT (target_store_id) WHERE target_store_id IS NOT NULL
DO UPDATE SET
    source               = EXCLUDED.source,
    source_reference     = EXCLUDED.source_reference,
    granted_capabilities = EXCLUDED.granted_capabilities,
    effective_from       = EXCLUDED.effective_from,
    effective_until      = EXCLUDED.effective_until,
    approved_by_admin_id = EXCLUDED.approved_by_admin_id,
    approved_at          = EXCLUDED.approved_at,
    revoked_at           = NULL,
    revoked_by_admin_id  = NULL,
    revoke_reason        = NULL,
    updated_at           = EXCLUDED.updated_at
WHERE delegation.seller_delegation_authorizations.revoked_at IS NOT NULL
RETURNING *;

-- name: UpdateDelegationAuthorization :one
-- 調整有效授權的範圍、依據與期限（全量取代）。CAS 述詞與 Grant 相反：兩者分開，過時的畫面按「調整範圍」
-- 不會把已撤回的店靜默地重新開通。
UPDATE delegation.seller_delegation_authorizations
SET source = sqlc.arg(source),
    source_reference = sqlc.arg(source_reference),
    granted_capabilities = sqlc.arg(granted_capabilities)::text[],
    effective_until = sqlc.narg(effective_until),
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)::bigint AND revoked_at IS NULL
RETURNING *;

-- name: RevokeDelegationAuthorization :one
-- 撤回：三欄一起寫。沒有列表示已經撤回，呼叫方回既有的列，不覆蓋第一次的值、不補寫第二列歷史。
UPDATE delegation.seller_delegation_authorizations
SET revoked_at = sqlc.arg(now)::timestamptz,
    revoked_by_admin_id = sqlc.arg(revoked_by_admin_id)::bigint,
    revoke_reason = sqlc.arg(revoke_reason)::text,
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)::bigint AND revoked_at IS NULL
RETURNING *;

-- name: InsertDelegationAuthorizationAuditLog :one
-- 治理歷史唯一的寫入，與授權列同一個 transaction。同一列授權上同一把冪等鍵已經有紀錄時沒有列回傳（呼叫方回
-- 9267 並 rollback）；推斷子句指名 partial index，主鍵衝突仍然報錯。沒有冪等鍵的寫入不去重。
INSERT INTO delegation.delegation_authorization_audit_logs (
    id, authorization_id, target_store_id, action, actor_admin_id, actor_display_name, operation_reason,
    source, source_reference, capabilities_before, capabilities_after,
    effective_until_before, effective_until_after, idempotency_key, request_id, correlation_id, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(authorization_id), sqlc.arg(target_store_id), sqlc.arg(action), sqlc.arg(actor_admin_id),
    sqlc.arg(actor_display_name), sqlc.arg(operation_reason),
    sqlc.narg(source), sqlc.narg(source_reference), sqlc.narg(capabilities_before)::text[],
    sqlc.narg(capabilities_after)::text[],
    sqlc.narg(effective_until_before), sqlc.narg(effective_until_after), sqlc.narg(idempotency_key),
    sqlc.narg(request_id), sqlc.narg(correlation_id), sqlc.arg(now)::timestamptz
)
ON CONFLICT (authorization_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: GetDelegationAuthorizationAuditLogByIdempotencyKey :one
SELECT * FROM delegation.delegation_authorization_audit_logs
WHERE authorization_id = sqlc.arg(authorization_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: ListDelegationAuthorizationAuditLogs :many
-- 單店的治理歷史，新到舊；(created_at, id) 讓同一時刻的列在分頁邊界穩定。
SELECT * FROM delegation.delegation_authorization_audit_logs
WHERE target_store_id = sqlc.arg(target_store_id)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationAuthorizationAuditLogs :one
SELECT count(*) FROM delegation.delegation_authorization_audit_logs WHERE target_store_id = $1;
