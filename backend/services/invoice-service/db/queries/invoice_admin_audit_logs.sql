-- invoice_admin_audit_logs｜ADM 操作稽核
-- SSOT：設計書 58 號 §5.1.13
-- 本波放骨架 ＋ 一支 append。reason_code／reason_detail 都是 NOT NULL——**那個 NOT NULL 就是
-- 「強制填理由」這條規則的執行者**，不要在應用層再造一份可被繞過的檢查。

-- name: InsertInvoiceAdminAuditLog :one
-- uq_invadmaudit_client_request（actor_account_id, action, request_id）WHERE request_id IS NOT NULL
-- ⇒ 同一個 ADM 對同一動作重送同一 request_id 只留一列。
-- ON CONFLICT DO NOTHING 不指名 target（本表只有這一個唯一索引，但不指名一樣正確，
-- 且未來加第二個唯一約束時不會突然開始拋 23505）。
--
-- 🔴 回 id 而不是 rows：回應的 auditLogId 是 ADM 連回稽核列的唯一線索。本句原本是
-- :execrows，於是那個欄位恆為 "1"——**一個長得像合法 id 的錯值**，ADM 拿去查只會
-- 查不到而不知道為什麼。重送（衝突）時 INSERT 不回列，所以用 UNION ALL 從既有列
-- 撈回同一個 id：重送要拿到的正是第一次那一列，不是新的一列。
WITH ins AS (
INSERT INTO invoice_admin_audit_logs (
  actor_account_id, action, resource_type, resource_id, before_state, after_state,
  reason_code, reason_detail, approval_actor_account_id,
  trace_id, request_id, correlation_id, client_ip, user_agent, occurred_at)
VALUES (
  sqlc.arg(actor_account_id), sqlc.arg(action), sqlc.arg(resource_type), sqlc.arg(resource_id),
  sqlc.narg(before_state), sqlc.narg(after_state),
  sqlc.arg(reason_code), sqlc.arg(reason_detail), sqlc.narg(approval_actor_account_id),
  sqlc.narg(trace_id), sqlc.narg(request_id), sqlc.narg(correlation_id),
  sqlc.narg(client_ip), sqlc.narg(user_agent), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING id
)
SELECT id FROM ins
UNION ALL
SELECT id FROM invoice_admin_audit_logs
WHERE sqlc.narg(request_id)::text IS NOT NULL
  AND actor_account_id = sqlc.arg(actor_account_id)
  AND action = sqlc.arg(action)
  AND request_id = sqlc.narg(request_id)
LIMIT 1;

-- name: ListInvoiceAdminAuditLogsByResource :many
SELECT * FROM invoice_admin_audit_logs
WHERE resource_type = sqlc.arg(resource_type) AND resource_id = sqlc.arg(resource_id)
ORDER BY occurred_at DESC
LIMIT sqlc.arg(row_limit);

-- name: ListInvoiceAuditLogsForAdmin :many
-- 端點 61（A10 稽核查詢），九個 filter。
--
-- 🔴 invoice_admin_audit_logs 只有 (resource_type, resource_id) 這一對多型指標，
--    **沒有 store_id／order_no／operation_type 欄**。那三個 filter 因此只能靠把
--    resource_id 解析回實體再 join；五條 LEFT JOIN 各自被 resource_type 夾住，
--    沒有任何一條會在 resource_type 不符時誤配到別張表的同號 id
--    （bigserial 與 TSID 混用 ⇒ 不夾 resource_type 幾乎必然誤配）。
--    resource_type 的字面值由呼叫端傳入（proto `target_kind` 五值：
--    TAX_PROFILE／PROVIDER_ACCOUNT／JOB／DOCUMENT／COMPLAINT），不寫死在此。
--
-- document 這一條走 COALESCE 三選一：直接指向 DOCUMENT 的、經 JOB 帶出來的、
--    經 COMPLAINT 帶出來的。issuer 再由 document／tax_profile／provider_account
--    三者擇一解析 ⇒ store_id filter 對五種 resource_type 都成立。
--
-- ⚠️ `before_state`／`after_state` 的遮罩在**寫入時**完成（§6.1.14）：查詢時遮罩
--    表示 DB 裡存著明文，而本表保存 5 年 ＝ 一個五年期的明文 PII 倉庫。本查詢
--    原樣回傳，不做任何遮罩。
SELECT sqlc.embed(l),
       d.source_ref     AS document_source_ref,
       d.id             AS resolved_document_id,
       o.id             AS resolved_operation_id,
       o.operation_type AS resolved_operation_type,
       i.issuer_ref_id  AS issuer_ref_id
FROM invoice_admin_audit_logs l
LEFT JOIN invoice_operations o
       ON l.resource_type = sqlc.arg(job_resource_type)::text AND o.id = l.resource_id
LEFT JOIN invoice_complaints cp
       ON l.resource_type = sqlc.arg(complaint_resource_type)::text AND cp.id = l.resource_id
LEFT JOIN seller_tax_profiles tp
       ON l.resource_type = sqlc.arg(tax_profile_resource_type)::text AND tp.id = l.resource_id
LEFT JOIN invoice_provider_accounts pa
       ON l.resource_type = sqlc.arg(provider_account_resource_type)::text AND pa.id = l.resource_id
LEFT JOIN invoice_documents d
       ON d.id = COALESCE(
            CASE WHEN l.resource_type = sqlc.arg(document_resource_type)::text
                 THEN l.resource_id END,
            o.document_id, cp.document_id)
LEFT JOIN invoice_issuers i
       ON i.id = COALESCE(d.issuer_id, tp.issuer_id, pa.issuer_id)
WHERE (NOT sqlc.arg(has_target_actor)::boolean
       OR l.actor_account_id = sqlc.arg(target_actor_account_id)::bigint)
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(has_document_id)::boolean OR d.id = sqlc.arg(document_id)::bigint)
  AND (NOT sqlc.arg(has_job_id)::boolean      OR o.id = sqlc.arg(job_id)::bigint)
  AND (sqlc.arg(operation_type_filter)::text = ''
       OR o.operation_type = sqlc.arg(operation_type_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR l.occurred_at >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR l.occurred_at <= sqlc.arg(date_to)::timestamptz)
ORDER BY l.occurred_at DESC, l.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountInvoiceAuditLogsForAdmin :one
-- 端點 61 的 total。**五條 LEFT JOIN 與述詞必須與 List 逐字相同**：少一條 join
-- 就少一種 resource_type 解析得出 store_id／order_no，total 會比 records 少。
SELECT COUNT(*)::bigint
FROM invoice_admin_audit_logs l
LEFT JOIN invoice_operations o
       ON l.resource_type = sqlc.arg(job_resource_type)::text AND o.id = l.resource_id
LEFT JOIN invoice_complaints cp
       ON l.resource_type = sqlc.arg(complaint_resource_type)::text AND cp.id = l.resource_id
LEFT JOIN seller_tax_profiles tp
       ON l.resource_type = sqlc.arg(tax_profile_resource_type)::text AND tp.id = l.resource_id
LEFT JOIN invoice_provider_accounts pa
       ON l.resource_type = sqlc.arg(provider_account_resource_type)::text AND pa.id = l.resource_id
LEFT JOIN invoice_documents d
       ON d.id = COALESCE(
            CASE WHEN l.resource_type = sqlc.arg(document_resource_type)::text
                 THEN l.resource_id END,
            o.document_id, cp.document_id)
LEFT JOIN invoice_issuers i
       ON i.id = COALESCE(d.issuer_id, tp.issuer_id, pa.issuer_id)
WHERE (NOT sqlc.arg(has_target_actor)::boolean
       OR l.actor_account_id = sqlc.arg(target_actor_account_id)::bigint)
  AND (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(has_document_id)::boolean OR d.id = sqlc.arg(document_id)::bigint)
  AND (NOT sqlc.arg(has_job_id)::boolean      OR o.id = sqlc.arg(job_id)::bigint)
  AND (sqlc.arg(operation_type_filter)::text = ''
       OR o.operation_type = sqlc.arg(operation_type_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR l.occurred_at >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR l.occurred_at <= sqlc.arg(date_to)::timestamptz);
