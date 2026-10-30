-- 代行的稽核（兩階段：intent 寫 PENDING，confirm 回填結果）。

-- name: InsertDelegationAuditLog :one
-- 唯一的稽核 INSERT。notify_seller 一律明寫。同一個 session 的同一把冪等鍵已經有列時沒有列回傳（呼叫方改讀
-- 既有的列，回報重放）；推斷子句指名 partial index，主鍵衝突仍然報錯。
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, operation_reason, case_reference,
    request_id, correlation_id, idempotency_key, payload_hash,
    before_snapshot, resource_version_before, user_agent,
    risk_level, notify_seller, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    sqlc.arg(origin), sqlc.arg(capability), sqlc.arg(action), sqlc.arg(resource_type), sqlc.narg(resource_id),
    sqlc.narg(resource_name),
    'PENDING', sqlc.narg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.narg(idempotency_key), sqlc.narg(payload_hash),
    sqlc.narg(before_snapshot), sqlc.narg(resource_version_before), sqlc.narg(user_agent),
    sqlc.arg(risk_level), sqlc.arg(notify_seller), sqlc.arg(now)::timestamptz
)
ON CONFLICT (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: GetDelegationAuditLog :one
SELECT * FROM delegation.admin_operation_audit_logs WHERE id = $1;

-- name: GetDelegationAuditLogByIdempotencyKey :one
SELECT * FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: LatestDelegationAuditReason :one
-- 排程執行繼承的操作理由：同一個 session、同一個資源、建立排程的那個動作最新的一列。不過濾 result：理由在
-- intent 當下就寫定了，confirm 沒送到而停在 PENDING 的排程也是合法的。
SELECT operation_reason FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id) AND resource_id = sqlc.arg(resource_id)::text AND action = sqlc.arg(action)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ConfirmDelegationAuditLog :one
-- confirm：CAS 的述詞是 result = 'PENDING'，第二次 confirm 沒有列、既有的結論不被覆蓋。resource_id 以 COALESCE
-- 保留 intent 寫入的值（建立類在這裡才補上）。成功而且要通知賣家時，完成事件在同一個 transaction 內排入送出，
-- notified_at 同時寫入。
UPDATE delegation.admin_operation_audit_logs
SET result = sqlc.arg(result),
    resource_id = COALESCE(sqlc.narg(resource_id), resource_id),
    after_snapshot = sqlc.narg(after_snapshot),
    changed_fields = sqlc.narg(changed_fields),
    resource_version_after = sqlc.narg(resource_version_after),
    biz_code = sqlc.narg(biz_code),
    confirmed_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    notified_at = CASE
        WHEN sqlc.arg(result)::text = 'SUCCEEDED' AND notify_seller
        THEN date_trunc('second', sqlc.arg(now)::timestamptz)
    END
WHERE id = sqlc.arg(id) AND result = 'PENDING'
RETURNING *;

-- name: ListDelegationAuditLogs :many
-- 稽核時間線（單一 session）與跨 session 的稽核清單共用：session、管理員、店家都是選填的軸，其他篩選的空字串
-- 表示不篩選（值域由呼叫方先驗）。排序是 (created_at, id)：批次操作在同一個請求內逐項寫列，created_at 可能相同。
SELECT * FROM delegation.admin_operation_audit_logs
WHERE (sqlc.narg(session_id)::bigint IS NULL OR session_id = sqlc.narg(session_id)::bigint)
  AND (sqlc.narg(actor_admin_id)::bigint IS NULL OR actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text)
  AND (sqlc.arg(resource_type)::text = '' OR resource_type = sqlc.arg(resource_type)::text)
  AND (sqlc.arg(result)::text = '' OR result = sqlc.arg(result)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR created_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR created_at <= sqlc.narg(date_to)::timestamptz)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationAuditLogs :one
-- 與 ListDelegationAuditLogs 相同的篩選：兩者分岔時 total 與 records 屬於不同的母體。
SELECT count(*) FROM delegation.admin_operation_audit_logs
WHERE (sqlc.narg(session_id)::bigint IS NULL OR session_id = sqlc.narg(session_id)::bigint)
  AND (sqlc.narg(actor_admin_id)::bigint IS NULL OR actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(action)::text = '' OR action = sqlc.arg(action)::text)
  AND (sqlc.arg(resource_type)::text = '' OR resource_type = sqlc.arg(resource_type)::text)
  AND (sqlc.arg(result)::text = '' OR result = sqlc.arg(result)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR created_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR created_at <= sqlc.narg(date_to)::timestamptz);

-- name: DelegationAuditSummary :one
-- 結束摘要的計數。PENDING 不算成功。notified 數的是事實（notified_at），不是意圖（notify_seller）。履約、金流的
-- 計數以子字串比對 action、刻意不夾 result：一列 PENDING 的履約 intent 就已經證明封鎖被打穿；也不能改成列舉
-- 已登錄的動作（要偵測的東西按定義不在清單裡）。
SELECT count(*) FILTER (WHERE result = 'SUCCEEDED')                         AS succeeded_count,
       count(*) FILTER (WHERE risk_level = 'HIGH' AND result = 'SUCCEEDED') AS high_risk_count,
       count(*) FILTER (WHERE result = 'PENDING')                           AS pending_count,
       count(*) FILTER (WHERE notified_at IS NOT NULL)                      AS notified_count,
       count(*) FILTER (WHERE lower(action) LIKE ANY (sqlc.arg(fulfillment_patterns)::text[])) AS fulfillment_count,
       count(*) FILTER (WHERE lower(action) LIKE ANY (sqlc.arg(finance_patterns)::text[]))     AS finance_count
FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id);
