-- 買家配額（月/週期）持久層——procurement_usage／procurement_usage_reservations
-- 的 reserve/confirm/release，比照 services/ai-service/db/queries（ai_usage 家族）
-- 同一套既定模式，只是換一組計量軸（buyer+period 而非 store+week）。

-- name: AcquireProcurementUsageLock :exec
-- 同 services/ai-service/db/queries/usage.sql 的 AcquireAiUsageLock：
-- pg_advisory_xact_lock 只有 (bigint) 與 (int, int) 兩種簽章，無 (int, bigint)；
-- buyer_id 是 TSID（64-bit，可能超出 int4 範圍），不可硬塞進兩參數版；改用
-- hashtextextended(text, bigint) 把 'procurement_usage_buyer' 命名空間與 buyer_id
-- 一起雜湊成單一 bigint key。
-- 🔴 advisory lock 的命名空間是 per-database：hawkr_ai 同時被本查詢
-- （key=buyer_id）與 AcquireAiUsageLock（key=store_id）使用，裸 bigint 會讓
-- 兩個子系統互相撞鎖。2026-09-19 實查 dev 交集見 usage.sql 同段註解。
SELECT pg_advisory_xact_lock(hashtextextended('procurement_usage_buyer', $1::bigint));

-- name: GetProcurementUsageByBuyerAndPeriod :one
SELECT * FROM procurement_usage WHERE buyer_id = $1 AND period_start = $2;

-- name: CountPendingProcurementUsageReservations :one
SELECT COUNT(*) FROM procurement_usage_reservations
WHERE buyer_id = $1 AND period_start = $2 AND expires_at > now();

-- name: DeleteExpiredProcurementUsageReservations :exec
DELETE FROM procurement_usage_reservations WHERE expires_at <= now();

-- name: CreateProcurementUsageReservation :exec
INSERT INTO procurement_usage_reservations (id, buyer_id, period_start, expires_at)
VALUES ($1, $2, $3, $4);

-- name: DeleteProcurementUsageReservationByID :exec
DELETE FROM procurement_usage_reservations WHERE id = $1;

-- name: IncrementProcurementUsage :exec
INSERT INTO procurement_usage (buyer_id, period_start, usage_count, updated_at)
VALUES ($1, $2, 1, now())
ON CONFLICT (buyer_id, period_start) DO UPDATE SET usage_count = procurement_usage.usage_count + 1, updated_at = now();
