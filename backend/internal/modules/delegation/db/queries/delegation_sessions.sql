-- 代行的 session 的 canonical 查詢。時間一律由模組傳入（now、idle_since = now − 閒置逾時），
-- 不用 DB 的 now()：測試注入時鐘，也讓同一個請求的判定都用同一個時刻。
--
-- 「進行中」的述詞有四項：status = 'ACTIVE'、ended_at IS NULL、now < expires_at、last_active_at > idle_since。
-- DB 的 status 沒有 expired（到期在讀取時推導），少了後兩項，一個已經 hard 或閒置到期、但還沒被收斂的列會被
-- 當成進行中：衝突探測把它報成 9268／9269，兌換照樣交出 credential，撤回授權的確認框多數它一次。

-- name: InsertDelegationSession :one
-- 建立 session 並簽發 handoff code（同一句）。last_active_at 等於 issued_at。ON CONFLICT DO NOTHING 涵蓋每一個
-- unique index：沒有列時由呼叫方在同一個 transaction 內判斷撞到哪一個（冪等鍵、管理員或店家的進行中、
-- display_code），不接 23505（Postgres 的 transaction 會中止，PostgreSQL 設計規範 §十二）。
INSERT INTO delegation.delegated_operation_sessions (
    display_code, handoff_code_hash, handoff_expires_at,
    actor_admin_id, actor_display_name, target_store_id, target_seller_account_id, target_store_name,
    reason_code, reason_label, case_reference, capabilities,
    issued_at, expires_at, last_active_at, status,
    idempotency_key, create_request_id, create_correlation_id, create_user_agent,
    created_at, updated_at
) VALUES (
    sqlc.arg(display_code), sqlc.arg(handoff_code_hash),
    sqlc.arg(handoff_expires_at)::timestamptz,
    sqlc.arg(actor_admin_id), sqlc.arg(actor_display_name), sqlc.arg(target_store_id),
    sqlc.arg(target_seller_account_id), sqlc.arg(target_store_name),
    sqlc.arg(reason_code), sqlc.arg(reason_label), sqlc.narg(case_reference), sqlc.arg(capabilities)::text[],
    sqlc.arg(issued_at)::timestamptz, sqlc.arg(expires_at)::timestamptz, sqlc.arg(issued_at)::timestamptz, 'ACTIVE',
    sqlc.arg(idempotency_key), sqlc.arg(request_id), sqlc.arg(correlation_id),
    sqlc.narg(user_agent), sqlc.arg(now)::timestamptz, sqlc.arg(now)::timestamptz
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: GetDelegationSession :one
SELECT * FROM delegation.delegated_operation_sessions WHERE id = $1;

-- name: GetDelegationSessionByIdempotencyKey :one
-- 冪等重放的入口：命中就回既有的列，尚未兌換時才改簽 handoff code。
SELECT * FROM delegation.delegated_operation_sessions
WHERE actor_admin_id = sqlc.arg(actor_admin_id) AND idempotency_key = sqlc.arg(idempotency_key);

-- name: GetDelegationSessionByCredential :one
-- session 端點的 credential 反查。必須是純 SELECT：GET current 每 30 秒輪詢，輪詢不能算活動。
SELECT * FROM delegation.delegated_operation_sessions WHERE credential_hash = $1;

-- name: TouchDelegationSession :one
-- 代行資源路由的 credential 反查，同一句順延閒置逾時。只動 last_active_at（動 revision 會讓前端對帳失敗，動
-- expires_at 就是 sliding expiration）。條件放在 CASE 裡而不是 WHERE：終態的 session 也要回傳，呼叫方才回得出
-- expired、revoked；條件要含閒置與 hard 到期，否則下一個請求會把已經逾時的 session 救回來。
UPDATE delegation.delegated_operation_sessions
SET last_active_at = CASE
        WHEN status = 'ACTIVE' AND ended_at IS NULL
             AND sqlc.arg(now)::timestamptz < expires_at
             AND last_active_at > sqlc.arg(idle_since)::timestamptz
        THEN date_trunc('second', sqlc.arg(now)::timestamptz)
        ELSE last_active_at
    END
WHERE credential_hash = sqlc.arg(credential_hash)
RETURNING *;

-- name: GetActiveDelegationSessionOfActor :one
-- preflight 的衝突探測。管理員與店家拆成兩句（OR 會讓兩個 partial unique index 都用不上）。
SELECT * FROM delegation.delegated_operation_sessions
WHERE actor_admin_id = sqlc.arg(actor_admin_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: GetActiveDelegationSessionOfStore :one
SELECT * FROM delegation.delegated_operation_sessions
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: DelegationActorSlotTaken :one
-- 建立時沒有寫入的判斷：述詞與 uidx_delegated_operation_sessions_active_actor 相同。
SELECT EXISTS (
    SELECT 1 FROM delegation.delegated_operation_sessions
    WHERE actor_admin_id = $1 AND status = 'ACTIVE' AND ended_at IS NULL
) AS taken;

-- name: DelegationStoreSlotTaken :one
SELECT EXISTS (
    SELECT 1 FROM delegation.delegated_operation_sessions
    WHERE target_store_id = $1 AND status = 'ACTIVE' AND ended_at IS NULL
) AS taken;

-- name: CountActiveDelegationSessionsOfStore :one
-- 撤回授權的確認框要顯示的「將被中止的 session 數」；述詞與連帶撤銷那句相同。
SELECT count(*) FROM delegation.delegated_operation_sessions
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz;

-- name: EndStaleDelegationSessions :execrows
-- 建立前在同一個 transaction 內結束這位管理員與這家店已經到期（hard 或閒置）、卻還佔著 partial unique index
-- 的列。兩個到期述詞都要寫：少了閒置那一個，被閒置踢出的管理員重建時會撞 9268／9269。
UPDATE delegation.delegated_operation_sessions
SET ended_at = sqlc.arg(now)::timestamptz, updated_at = sqlc.arg(now)::timestamptz
WHERE status = 'ACTIVE' AND ended_at IS NULL
  AND (expires_at <= sqlc.arg(now)::timestamptz OR last_active_at <= sqlc.arg(idle_since)::timestamptz)
  AND (actor_admin_id = sqlc.arg(actor_admin_id) OR target_store_id = sqlc.arg(target_store_id));

-- name: EndDelegationSession :execrows
-- 主動結束：expires_at 壓到截秒的 now，但至少比 issued_at 晚一秒（前端要求 expiresAt − issuedAt > 0）。
UPDATE delegation.delegated_operation_sessions
SET ended_at = sqlc.arg(now)::timestamptz,
    expires_at = GREATEST(issued_at + INTERVAL '1 second',
                          LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz))),
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND status = 'ACTIVE' AND ended_at IS NULL;

-- name: RedeemDelegationHandoff :one
-- handoff 兌換：單句 CAS，七個述詞（code 自己的兩個、尚未核發 credential、session 被終止的兩個、到期的兩個）。沒有列一律是
-- 9282，四種成因不區分（區分等於給一支「這個 code 存在嗎」的探測器）。同一句寫入新憑證的 hash 並清掉 code，
-- 不寫稽核、不順延 last_active_at、不改 revision。
UPDATE delegation.delegated_operation_sessions
SET handoff_code_hash = NULL, credential_hash = sqlc.arg(credential_hash)
WHERE handoff_code_hash = sqlc.arg(handoff_code_hash)
  AND handoff_expires_at > sqlc.arg(now)::timestamptz
  AND credential_hash IS NULL
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING *;

-- name: ReissueDelegationHandoff :one
-- 尚未兌換時的冪等重放改簽 handoff code；已核發 credential 的列不可重簽。
-- 重放不是狀態轉換，不動 revision。
UPDATE delegation.delegated_operation_sessions
SET handoff_code_hash = sqlc.arg(handoff_code_hash), handoff_expires_at = sqlc.arg(handoff_expires_at)::timestamptz
WHERE id = sqlc.arg(id) AND credential_hash IS NULL
RETURNING *;

-- name: RevokeDelegationSession :one
-- 治理的撤銷：單句 CAS；單筆撤銷只要求 revoked_at IS NULL（已到期或已結束的也能撤銷，留下治理痕跡）。沒有列表示已經
-- 撤銷過，呼叫方回傳既有的列（revoked_at 是舉證欄，不能每次不同）。ended_at 一起寫：已經結束的保留原本的時刻，
-- 已經到期的取 expires_at，否則摘要的時長會隨查詢時刻長大。only_active 用於撤回授權與管理員登出，
-- 只撤銷仍符合四項進行中述詞的列；治理的單筆撤銷不設此限制。
UPDATE delegation.delegated_operation_sessions
SET status = 'REVOKED',
    revoked_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    ended_at = COALESCE(ended_at, LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz))),
    revoke_reason = sqlc.arg(revoke_reason)::text,
    revoked_by_admin_id = sqlc.arg(revoked_by_admin_id)::bigint,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND revoked_at IS NULL
  AND (NOT sqlc.arg(only_active)::boolean OR (
    status = 'ACTIVE' AND ended_at IS NULL
    AND sqlc.arg(now)::timestamptz < expires_at
    AND last_active_at > sqlc.arg(idle_since)::timestamptz
  ))
RETURNING *;

-- name: RevokeActiveDelegationsOfActor :many
-- revoked_by_admin_id 可為 NULL：系統因角色變動而撤銷時沒有操作者，不填被撤銷者自己。
UPDATE delegation.delegated_operation_sessions
SET status = 'REVOKED',
    revoked_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    ended_at = LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz)),
    revoke_reason = sqlc.arg(revoke_reason)::text,
    revoked_by_admin_id = sqlc.narg(revoked_by_admin_id)::bigint,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE actor_admin_id = sqlc.arg(actor_admin_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING id;

-- name: RevokeActiveDelegationsOfStore :many
UPDATE delegation.delegated_operation_sessions
SET status = 'REVOKED',
    revoked_at = date_trunc('second', sqlc.arg(now)::timestamptz),
    ended_at = LEAST(expires_at, date_trunc('second', sqlc.arg(now)::timestamptz)),
    revoke_reason = sqlc.arg(revoke_reason)::text,
    revoked_by_admin_id = sqlc.arg(revoked_by_admin_id)::bigint,
    revision = revision + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE target_store_id = sqlc.arg(target_store_id)
  AND status = 'ACTIVE' AND ended_at IS NULL
  AND sqlc.arg(now)::timestamptz < expires_at
  AND last_active_at > sqlc.arg(idle_since)::timestamptz
RETURNING id;

-- name: ListDelegationSessions :many
-- 跨 session 的治理清單，新到舊。沒有 status 篩選：expired 是讀取時推導的，寫進 WHERE 就有兩份實作。時間區間
-- 比對 issued_at（對外顯示的是它）。分頁是 OFFSET：舊契約要回 page、page_size、total。
SELECT s.*
FROM delegation.delegated_operation_sessions s
WHERE (sqlc.narg(actor_admin_id)::bigint IS NULL OR s.actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR s.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(display_code)::text = '' OR s.display_code = sqlc.arg(display_code)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR s.issued_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR s.issued_at <= sqlc.narg(date_to)::timestamptz)
ORDER BY s.created_at DESC, s.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountDelegationSessions :one
-- 與 ListDelegationSessions 相同的篩選。
SELECT count(*) FROM delegation.delegated_operation_sessions s
WHERE (sqlc.narg(actor_admin_id)::bigint IS NULL OR s.actor_admin_id = sqlc.narg(actor_admin_id)::bigint)
  AND (sqlc.narg(target_store_id)::bigint IS NULL OR s.target_store_id = sqlc.narg(target_store_id)::bigint)
  AND (sqlc.arg(display_code)::text = '' OR s.display_code = sqlc.arg(display_code)::text)
  AND (sqlc.narg(date_from)::timestamptz IS NULL OR s.issued_at >= sqlc.narg(date_from)::timestamptz)
  AND (sqlc.narg(date_to)::timestamptz IS NULL OR s.issued_at <= sqlc.narg(date_to)::timestamptz);
