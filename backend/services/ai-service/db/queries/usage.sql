-- name: AcquireAiUsageLock :exec
-- reserveSlot／confirmSlot／releaseSlot 都要在同一個 transaction 內、對同一個
-- store_id 先取得這把鎖才能動 ai_usage／ai_usage_reservations，序列化同 store 的
-- 並發存取。advisory lock 會在 transaction commit／rollback 時自動釋放；呼叫端
-- 必須已經在交易內，否則鎖在這句執行完就立刻釋放，等於沒鎖。
-- 注意：pg_advisory_xact_lock 只有 (bigint) 與 (int, int) 兩種簽章，無 (int, bigint)。
-- store_id 是 TSID（64-bit，可能超出 int4 範圍），不可硬塞進兩參數版；改用
-- hashtextextended(text, bigint) 把 'ai_usage_store' 命名空間與 store_id 一起雜湊成
-- 單一 bigint key，對應單參數版 pg_advisory_xact_lock(bigint)。
-- 🔴 advisory lock 的命名空間是 per-database，不是 per-table／per-subsystem：
-- 這顆資料庫（hawkr_ai）同時被 ai_usage（本查詢，key=store_id）與
-- procurement_usage（AcquireProcurementUsageLock，key=buyer_id）使用。2026-09-19
-- 實查 dev：ai_usage.store_id 值域 1–7487768729113814280（26 列），
-- procurement_usage.buyer_id 值域 2636840881–7506715228034301952（248 列），
-- 交集恰好 1 個鍵：2636840881——裸 bigint 會讓那個賣家的修圖與那個買家的詢價
-- 互相排隊。前綴命名空間後兩邊各自落在不同雜湊空間，不再共用同一把鎖。
SELECT pg_advisory_xact_lock(hashtextextended('ai_usage_store', sqlc.arg(store_id)::bigint));

-- name: CountPendingAiUsageReservations :one
SELECT COUNT(*) FROM ai_usage_reservations
WHERE store_id = $1 AND week_start = $2 AND expires_at > NOW();

-- name: DeleteExpiredAiUsageReservations :exec
DELETE FROM ai_usage_reservations WHERE expires_at < NOW() - INTERVAL '1 day';

-- name: CreateAiUsageReservation :exec
INSERT INTO ai_usage_reservations (id, store_id, week_start, expires_at)
VALUES ($1, $2, $3, $4);

-- name: DeleteAiUsageReservationByID :exec
DELETE FROM ai_usage_reservations WHERE id = $1;

-- name: IncrementAiUsage :exec
INSERT INTO ai_usage (store_id, week_start, usage_count, created_at, updated_at)
VALUES ($1, $2, 1, NOW(), NOW())
ON CONFLICT (store_id, week_start)
DO UPDATE SET usage_count = ai_usage.usage_count + 1, updated_at = NOW();
