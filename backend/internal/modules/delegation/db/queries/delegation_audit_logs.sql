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

-- name: InsertScheduledPublishAuditLog :one
-- 排程到點發布的單段稽核：與貼文發布同一個 transaction 寫入，直接是 SUCCEEDED（沒有 intent 與 confirm 兩段，貼文發布的事實就是結果）。
-- 冪等鍵是 schedule:<排程 ID>；同一個 session 的同一把鍵已經有列時沒有列回傳（重送的事件，呼叫方當成功）。固定值：origin SCHEDULED、
-- capability posts.publish、action POST_PUBLISH、resource_type POST；confirmed_at 與 notified_at 與 confirm 一樣截秒；notify_seller 由呼叫方依排程動作的稽核列決定，為 true 才寫 notified_at。
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, operation_reason, case_reference,
    request_id, correlation_id, idempotency_key,
    risk_level, notify_seller, created_at, confirmed_at, notified_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    'SCHEDULED', 'posts.publish', 'POST_PUBLISH', 'POST', sqlc.arg(resource_id)::text, sqlc.narg(resource_name),
    'SUCCEEDED', sqlc.narg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.arg(idempotency_key)::text,
    sqlc.arg(risk_level), sqlc.arg(notify_seller)::boolean, sqlc.arg(now)::timestamptz, date_trunc('second', sqlc.arg(now)::timestamptz),
    CASE WHEN sqlc.arg(notify_seller)::boolean THEN date_trunc('second', sqlc.arg(now)::timestamptz) END
)
ON CONFLICT (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertSucceededAuditLog :one
-- 代行互動寫入的單段稽核：與被稽核的寫入同一個 transaction，直接是 SUCCEEDED（沒有 intent 與 confirm 兩段；寫入失敗時 transaction rollback，這一列跟著消失）。
-- 冪等鍵是呼叫方的 request_id；同一個 session 的同一把鍵已經有列時沒有列回傳（呼叫方讀既有的列判斷是重送還是內容不同）。origin 固定 INTERACTIVE；
-- confirmed_at 與 notified_at 與 confirm 一樣截秒；notify_seller 為 true 才寫 notified_at。建立類在寫入前還沒有資源 ID，resource_id 為 NULL，由 SetAuditLogOutcome 補上。
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, operation_reason, case_reference,
    request_id, correlation_id, idempotency_key, payload_hash,
    resource_version_before, user_agent,
    risk_level, notify_seller, created_at, confirmed_at, notified_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    'INTERACTIVE', sqlc.arg(capability), sqlc.arg(action), sqlc.arg(resource_type), sqlc.narg(resource_id)::text, sqlc.narg(resource_name),
    'SUCCEEDED', sqlc.arg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.arg(idempotency_key)::text, sqlc.arg(payload_hash),
    sqlc.narg(resource_version_before), sqlc.narg(user_agent),
    sqlc.arg(risk_level), sqlc.arg(notify_seller)::boolean, sqlc.arg(now)::timestamptz, date_trunc('second', sqlc.arg(now)::timestamptz),
    CASE WHEN sqlc.arg(notify_seller)::boolean THEN date_trunc('second', sqlc.arg(now)::timestamptz) END
)
ON CONFLICT (session_id, idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
RETURNING *;

-- name: InsertFailedAuditLog :one
-- 代行互動請求的失敗紀錄：被拒絕（能力不足、session 已失效、店家不可代行，含讀取）或寫入的業務失敗。在獨立的 transaction 寫入（被稽核的寫入若已 rollback，這一列仍要留下）。
-- 直接是 FAILED，沒有冪等鍵（失敗的嘗試不占用 request_id，同一個 request_id 重送仍可成功）；origin 固定 INTERACTIVE；notify_seller 固定 false；
-- confirmed_at 與 confirm 一樣截秒；failure_reason 是拒絕的代碼或 ErrorReason 的值名（沒有原因是 INTERNAL），新路徑不寫 biz_code。
INSERT INTO delegation.admin_operation_audit_logs (
    id, session_id, actor_admin_id, actor_display_name,
    target_store_id, target_seller_account_id, target_store_name,
    origin, capability, action, resource_type, resource_id, resource_name,
    result, failure_reason, operation_reason, case_reference,
    request_id, correlation_id, resource_version_before, user_agent,
    risk_level, notify_seller, created_at, confirmed_at
) VALUES (
    sqlc.arg(id), sqlc.arg(session_id), sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name),
    sqlc.arg(target_store_id), sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    'INTERACTIVE', sqlc.arg(capability), sqlc.arg(action), sqlc.arg(resource_type), sqlc.narg(resource_id)::text, sqlc.narg(resource_name),
    'FAILED', sqlc.arg(failure_reason)::text, sqlc.narg(operation_reason), sqlc.narg(case_reference),
    sqlc.arg(request_id), sqlc.arg(correlation_id), sqlc.narg(resource_version_before), sqlc.narg(user_agent),
    sqlc.arg(risk_level), false, sqlc.arg(now)::timestamptz, date_trunc('second', sqlc.arg(now)::timestamptz)
)
RETURNING *;

-- name: SetAuditLogOutcome :one
-- 寫入完成後補上稽核列的資源：建立類在這裡才有貼文 ID（已有值的保留），並記下資源名稱與寫入後的版本。只動這一列。
UPDATE delegation.admin_operation_audit_logs
SET resource_id = COALESCE(resource_id, sqlc.narg(resource_id)::text),
    resource_name = COALESCE(sqlc.narg(resource_name), resource_name),
    resource_version_after = sqlc.narg(resource_version_after)
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: GetDelegationAuditLog :one
SELECT * FROM delegation.admin_operation_audit_logs WHERE id = $1;

-- name: GetDelegationAuditLogByIdempotencyKey :one
SELECT * FROM delegation.admin_operation_audit_logs
WHERE session_id = sqlc.arg(session_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: LatestDelegationAuditReason :one
-- 排程執行繼承的操作理由與是否通知賣家：同一個 session、同一個資源、建立排程的那個動作最新的一列。不過濾 result：兩者在
-- intent 當下就寫定了，confirm 沒送到而停在 PENDING 的排程也是合法的。
SELECT operation_reason, notify_seller FROM delegation.admin_operation_audit_logs
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
    failure_reason = sqlc.narg(failure_reason),
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

-- name: InsertDelegationAuditLogItem :exec
-- 批次稽核列的一個商品；與父列同 transaction（RecordOperation）寫入。
INSERT INTO delegation.admin_operation_audit_log_items (audit_log_id, resource_id, resource_name, resource_version_before, resource_version_after)
VALUES (sqlc.arg(audit_log_id), sqlc.arg(resource_id), sqlc.narg(resource_name), sqlc.narg(resource_version_before), sqlc.narg(resource_version_after));

-- name: ListDelegationAuditLogItems :many
-- 批次稽核列的商品清單，依資源 ID 由小到大。
SELECT * FROM delegation.admin_operation_audit_log_items
WHERE audit_log_id = sqlc.arg(audit_log_id)
ORDER BY resource_id;
