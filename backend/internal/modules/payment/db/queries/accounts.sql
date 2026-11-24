-- name: PaymentAccountByStore :one
SELECT *
FROM payment.accounts
WHERE store_id = $1;

-- name: PaymentAccountByExternal :one
SELECT *
FROM payment.accounts
WHERE provider = $1 AND external_account_id = $2;

-- name: LockPaymentAccount :one
-- 鎖定帳戶列序列化它的寫入（全域鎖順序 payment 先於 seller：同步的 transaction 先鎖這裡，再經 PayoutAccountChanged 讓 seller 鎖它的列）。
SELECT *
FROM payment.accounts
WHERE store_id = $1
FOR UPDATE;

-- name: InsertPaymentAccount :execrows
-- seller 核准時在它的 transaction 內寫「建立中」的列（沒有外部 ID；聯絡信箱不存，建立外部帳戶時才讀）。一個擁有者一列，關店再開沿用既有的列；
-- 既有的列不改（ON CONFLICT DO NOTHING）：核准的 transaction 已鎖住 seller 的列，再鎖下層 payment 的既有列會違反全域鎖順序（platform/lockorder），
-- 既有的列只由本模組的 job 與管理員的指令寫。
INSERT INTO payment.accounts (store_id, provider, requirement_collection, created_at, updated_at)
VALUES (sqlc.arg(store_id), sqlc.arg(provider), sqlc.arg(requirement_collection), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT (store_id) DO NOTHING;

-- name: MarkPaymentAccountCreateCalled :one
-- 呼叫金流商之前先記「已呼叫」的時刻（已記過的保留最早的）：之後列上就分得出「從沒呼叫」與「呼叫過、結果不明」（D59）。已完成建立與等待清理的列不寫。
UPDATE payment.accounts
SET create_called_at = COALESCE(create_called_at, sqlc.arg(now)::timestamptz),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: SetPaymentAccountExternal :one
-- 建立的第一步拿到外部 ID 就寫進列（不等第二步，D59）；狀態仍是 NOT_CREATED，第二步成功才算已建立。只在還沒有外部 ID 時寫（重送安全），等待清理的列不寫
-- （清理走 SetPaymentAccountExternalForPurge）。已呼叫的時刻必須先寫（約束 accounts_called_check）。
UPDATE payment.accounts
SET external_account_id = sqlc.arg(external_account_id),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: SetPaymentAccountExternalForPurge :one
-- 清理把冪等鍵重送或列出對帳取得的外部帳戶 ID 寫回已標記等待清理的列，讓之後的關閉與刪除走有外部 ID 的路；只在還沒有外部 ID 時寫，列保持未建立。
UPDATE payment.accounts
SET external_account_id = sqlc.arg(external_account_id),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND purge_pending_at IS NOT NULL
RETURNING *;

-- name: CompletePaymentAccountCreate :one
-- 建立的第二步成功：記下完成的時刻，狀態從 NOT_CREATED 進 ACTION_REQUIRED（新帳戶要賣家補資料，同步 job 重讀後覆寫）；「可以開通」「同步」「輪詢」都看這個時刻。
-- 只在有外部 ID、還沒完成、沒有等待清理時寫。
UPDATE payment.accounts
SET create_completed_at = sqlc.arg(now)::timestamptz,
    status = 'ACTION_REQUIRED',
    create_failed_at = NULL,
    create_refused_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NOT NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: RotatePaymentAccountIdempotencyKey :one
-- 建立結果不明、超過 IdempotencyRetention（冪等鍵保存期）的列，管理員確認金流商那邊沒有這家店的帳戶之後換新鍵（D63），已呼叫的時刻改成新鍵第一次呼叫之前的時刻。
-- 只在還沒有外部 ID、還沒完成、沒有等待清理時寫。
UPDATE payment.accounts
SET create_idempotency_key = gen_random_uuid(),
    create_called_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND external_account_id IS NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL
RETURNING *;

-- name: MarkPaymentAccountCreateFailed :execrows
-- 建立失敗：記下失敗的時刻（進後台待辦）；已完成建立的列不寫（有外部 ID 但第二步沒成功的半建立列照寫）。已記過失敗的列保留最早的失敗時刻（COALESCE）。
-- refused 為真（金流商明確拒絕）時同時記 create_refused_at（已記過的保留最早的），週期掃描不再為這列重送；已經建立失敗的列在 Resolve 遇到永久拒絕時也要寫它，
-- 否則掃描要重送到 7 天期限。refused 為假不動既有的 create_refused_at。
UPDATE payment.accounts
SET create_failed_at = COALESCE(create_failed_at, sqlc.arg(now)::timestamptz),
    create_refused_at = CASE WHEN sqlc.arg(refused)::boolean THEN COALESCE(create_refused_at, sqlc.arg(now)::timestamptz) ELSE create_refused_at END,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NULL AND purge_pending_at IS NULL;

-- name: ClearPaymentAccountCreateFailed :execrows
-- 管理員重試建立：只有建立失敗的列清空失敗時刻，由呼叫方同一個 transaction 重新排入建立 job。
UPDATE payment.accounts
SET create_failed_at = NULL,
    create_refused_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NULL AND create_failed_at IS NOT NULL;

-- name: ApplyPaymentSnapshot :one
-- 同步重讀外部帳戶之後整列覆寫快取；還沒完成建立（含半建立）的列不寫。讀取開始時間早於已存的 fetched_at 的不寫（較晚開始的讀取已經先寫入，較早開始的不能蓋掉
-- 較新的快取，D32）：first_activated_at 只在還是 NULL 而這次轉帳能力啟用時寫（第一次啟用，之後不改）。沒有回傳列時呼叫方不標記 inbox 事件、不發布 PayoutAccountChanged。fetched_at 是 NULL（還沒有同步過）一律寫。
UPDATE payment.accounts
SET status = sqlc.arg(status),
    transfers_active = sqlc.arg(transfers_active),
    payouts_active = sqlc.arg(payouts_active),
    requirements = sqlc.arg(requirements),
    requirements_deadline = sqlc.narg(requirements_deadline),
    first_activated_at = COALESCE(first_activated_at, CASE WHEN sqlc.arg(transfers_active)::boolean THEN sqlc.arg(now)::timestamptz END),
    status_reason = sqlc.narg(status_reason),
    fetched_at = sqlc.arg(fetched_at),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NOT NULL
  AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(fetched_at))
RETURNING *;

-- name: ClaimPaymentAccountSellerRead :one
-- 賣家觸發的重讀先搶讀取名額再呼叫 provider（D33）：已經完成建立、上次賣家觸發的讀取（seller_read_claimed_at，含失敗的與並行的）和上次寫入的讀取
-- （fetched_at）都早於 stale_before 才寫入 seller_read_claimed_at 並回傳列；沒有回傳列就是名額被占走（呼叫方不讀 provider，依 seller_read_claimed_at 與 fetched_at 回答成功或 resource_exhausted）。單一條件式 UPDATE，
-- 並行的呼叫由列鎖序列化，後到的重新檢查條件而拿不到。同步 job 不經過這裡。
UPDATE payment.accounts
SET seller_read_claimed_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND create_completed_at IS NOT NULL AND purge_pending_at IS NULL
  AND (seller_read_claimed_at IS NULL OR seller_read_claimed_at <= sqlc.arg(stale_before))
  AND (fetched_at IS NULL OR fetched_at <= sqlc.arg(stale_before))
RETURNING *;

-- name: PollablePaymentAccountStoreIDs :many
-- 輪詢補漏（等待清理的帳戶除外）：UNDER_REVIEW、ACTION_REQUIRED 的帳戶，DISABLED 但原因碼不是終局（rejected*、unsupported*、closed）的帳戶（restricted 加 contact_stripe 或 no_resolution
-- 可以被 Stripe 解除，掉了 webhook 時只有輪詢補得到；rejected、unsupported 與外部帳戶已關閉（closed）不會再啟用，不輪詢），依 store_id 分批。inbox 有舊的未處理事件的帳戶由
-- Sweep 經 platform inbox 的 StaleRoutes 另外找。
SELECT a.store_id
FROM payment.accounts a
WHERE (a.status IN ('UNDER_REVIEW', 'ACTION_REQUIRED')
       OR (a.status = 'DISABLED'
           AND (a.status_reason IS NULL
                OR (a.status_reason NOT LIKE 'rejected%' AND a.status_reason NOT LIKE 'unsupported%' AND a.status_reason <> 'closed'))))
  AND a.purge_pending_at IS NULL
  AND a.store_id > sqlc.arg(after_store_id)
ORDER BY a.store_id
LIMIT sqlc.arg(batch);

-- name: PaymentAccountDeadlines :many
-- 補件期限提醒（D23）：期限落在 (now, until] 內、已完成建立且沒有等待清理的帳戶，依 store_id 分批。
SELECT store_id, requirements_deadline
FROM payment.accounts
WHERE create_completed_at IS NOT NULL AND purge_pending_at IS NULL
  AND requirements_deadline > sqlc.arg(now)::timestamptz AND requirements_deadline <= sqlc.arg(until)::timestamptz
  AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: PaymentAccountStoreIDsByExternal :many
-- 外部帳戶 ID 對應的擁有者（inbox 的路由鍵是外部帳戶 ID）；半建立與等待清理的帳戶不排同步。
SELECT store_id
FROM payment.accounts
WHERE provider = sqlc.arg(provider) AND external_account_id = ANY(sqlc.arg(external_account_ids)::text[]) AND create_completed_at IS NOT NULL AND purge_pending_at IS NULL
ORDER BY store_id;

-- name: StalePaymentAccountCreateStoreIDs :many
-- 保底：核准後建立 job 沒有跑成（例如擁有者關閉期間被略過）而且沒有進入失敗的帳戶（等待清理的除外）；擁有者是否營運中由呼叫方經 Owners 確認後才補排建立。
SELECT store_id
FROM payment.accounts
WHERE create_completed_at IS NULL AND create_failed_at IS NULL AND purge_pending_at IS NULL
  AND updated_at < sqlc.arg(idle_before) AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: UnresolvedPaymentAccountStoreIDs :many
-- 週期掃描了結「呼叫過、沒有外部 ID」的列用（D63），不論擁有者是否營運中、是否已記建立失敗（金流商明確拒絕的列除外：create_refused_at，重送只會再被拒絕，等管理員重試）；等待清理的列由清理 job 處理，不在這裡。called_before 是「已呼叫」的時刻要早於它
-- （剛呼叫的建立 job 還在跑，不重複送）；expired_before 是 UnresolvedDeadline 的界線：早於它而且已進建立失敗的列已是終局，不再選；早於它但還沒進建立失敗的列要被選到，由 Resolve 標成終局。
SELECT store_id
FROM payment.accounts
WHERE external_account_id IS NULL AND create_called_at IS NOT NULL AND create_completed_at IS NULL AND purge_pending_at IS NULL AND create_refused_at IS NULL
  AND create_called_at <= sqlc.arg(called_before)::timestamptz
  AND NOT (create_failed_at IS NOT NULL AND create_called_at <= sqlc.arg(expired_before)::timestamptz)
  AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: DeletePaymentAccounts :many
-- 擁有者的個資清理：刪除這些擁有者的帳戶列，回傳有外部帳戶的列的 provider 與外部 ID（呼叫方把它們在 inbox 的未處理事件標為已處理）。
-- 外部帳戶要先由呼叫方關閉成功（或列沒有外部 ID），這裡不處理外部帳戶。
DELETE FROM payment.accounts
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[])
RETURNING provider, external_account_id;

-- name: MarkPaymentAccountPurgePending :execrows
-- 擁有者要求清理、還沒完成：記下時刻（已記過的保留最早的時刻），有沒有外部 ID 都寫。
UPDATE payment.accounts
SET purge_pending_at = COALESCE(purge_pending_at, sqlc.arg(now)),
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id);

-- name: PurgePendingPaymentAccountStoreIDs :many
-- 週期掃描重排「關閉後刪除」用：等待清理而且沒有進終局的帳戶，依 store_id 分批。
SELECT store_id
FROM payment.accounts
WHERE purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL AND store_id > sqlc.arg(after_store_id)
ORDER BY store_id
LIMIT sqlc.arg(batch);

-- name: BlockPaymentAccountPurge :execrows
-- 清理進終局（D59）：記下進終局的時刻與原因（已在終局的保留最早的），週期掃描不再重排，進後台待辦 PAYMENT_ACCOUNT_PURGE_STUCK。只有已標記等待清理的列才能進。
UPDATE payment.accounts
SET purge_blocked_at = sqlc.arg(now)::timestamptz,
    purge_blocked_reason = sqlc.arg(reason)::text,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_pending_at IS NOT NULL AND purge_blocked_at IS NULL;

-- name: RestartPaymentAccountPurge :one
-- 管理員「重試關閉」：只有終局的列清除終局，等待清理的時刻重設成現在（期限從重試起重新算，否則第一次暫時性的失敗就又進終局），由呼叫方同一個 transaction 重新排入清理 job。
UPDATE payment.accounts
SET purge_blocked_at = NULL,
    purge_blocked_reason = NULL,
    purge_pending_at = sqlc.arg(now)::timestamptz,
    purge_replay_requested_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_blocked_at IS NOT NULL
RETURNING *;

-- name: RequestPaymentAccountPurgeReplay :execrows
-- 管理員對清理終局（unresolved_creation）的列要求以同一把鍵重送：記下時刻，重送排隊中或進行中時後端拒絕「刪除對應」與再次「重試關閉」（K1）。只有終局的列才能記。
UPDATE payment.accounts
SET purge_replay_requested_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_blocked_at IS NOT NULL;

-- name: ClearPaymentAccountPurgeReplay :execrows
-- 重送結束（取回、失敗或不再適用，不論結果）：清除標記，管理員才又能刪除對應或重試關閉。
UPDATE payment.accounts
SET purge_replay_requested_at = NULL,
    updated_at = sqlc.arg(now)
WHERE store_id = sqlc.arg(store_id) AND purge_replay_requested_at IS NOT NULL;
