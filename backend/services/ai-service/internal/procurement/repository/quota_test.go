//go:build integration

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestReserveQuota_UnderLimit_Reserves(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	periodStart := time.Now().UTC().Truncate(24 * time.Hour)

	reservationID, reserved, confirmedBefore, err := repo.ReserveQuota(ctx, identity.BuyerID(900001), periodStart, 10)
	require.NoError(t, err)
	require.True(t, reserved)
	require.NotZero(t, reservationID)
	require.Equal(t, 0, confirmedBefore, "fresh buyer/period has no confirmed usage yet")
}

func TestReserveQuota_AtLimit_DoesNotReserve(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	periodStart := time.Now().UTC().Truncate(24 * time.Hour)
	buyerID := identity.BuyerID(900002)

	_, reserved, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 1)
	require.NoError(t, err)
	require.True(t, reserved, "first reservation under limit=1 must succeed")

	_, reserved2, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 1)
	require.NoError(t, err)
	require.False(t, reserved2, "second reservation must be rejected — limit already pending")
}

// TestReserveQuota_ConcurrentCallsOnlyOneReserves 用真正並發的 goroutine 打同一個
// buyer/period（limit=1），證明 AcquireProcurementUsageLock 這道 advisory lock不是
// 裝飾——TestReserveQuota_AtLimit_DoesNotReserve 循序呼叫兩次只驗證了「confirmed+
// pending 累加比較」這段邏輯本身對不對，就算把 AcquireProcurementUsageLock 那行整個
// 刪掉，循序呼叫一樣會過（兩次呼叫根本沒有機會互相搶）；只有真正併發、兩個 goroutine
// 同時讀到「目前 pending=0，還沒滿」再各自寫入，才踩得到鎖要擋的那個 race window
// （Task 9／12 review finding，比照 quote_test.go 的
// TestQuoteRepository_ConfirmInvite_ConcurrentCallsOnlyOneWins 同一個理由與同一套
// WaitGroup + Mutex 收集結果的寫法）。
//
// N 個 goroutine 同時對同一個 buyerID/periodStart、limit=1 呼叫 ReserveQuota，斷言
// 恰好一個 reserved=true、其餘 reserved=false，且全部呼叫都不回錯誤——不是靠 mock，
// 是真的打 testcontainers 起的 Postgres，跑真正的 pg_advisory_xact_lock。
func TestReserveQuota_ConcurrentCallsOnlyOneReserves(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	periodStart := time.Now().UTC().Truncate(24 * time.Hour)
	buyerID := identity.BuyerID(900006)

	const concurrency = 10
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		reserved int
		errs     []error
	)
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			_, ok, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 1)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if ok {
				reserved++
			}
		}()
	}
	wg.Wait()

	require.Empty(t, errs, "併發呼叫任何一個都不該回錯誤")
	require.Equal(t, 1, reserved, "%d 個併發呼叫在 limit=1 下必須恰好一個搶到，實際搶到 %d 個", concurrency, reserved)
}

func TestReleaseQuota_FreesThePendingSlot(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	periodStart := time.Now().UTC().Truncate(24 * time.Hour)
	buyerID := identity.BuyerID(900003)

	reservationID, reserved, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 1)
	require.NoError(t, err)
	require.True(t, reserved)

	repo.ReleaseQuota(ctx, reservationID, buyerID)

	_, reserved2, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 1)
	require.NoError(t, err)
	require.True(t, reserved2, "released reservation must free the slot")
}

// TestConfirmQuotaInTx_ConvertsReservationToUsage exercises confirmQuotaInTx
// directly (it is unexported and only gets wired into AppendRevision's
// transaction in Task 12 — see quota.go's doc comment). Called the same way
// its future caller will: from inside an already-open r.txManager.WithTx
// closure, passing that closure's own *gen.Queries.
func TestConfirmQuotaInTx_ConvertsReservationToUsage(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	periodStart := time.Now().UTC().Truncate(24 * time.Hour)
	buyerID := identity.BuyerID(900004)

	reservationID, reserved, confirmedBefore, err := repo.ReserveQuota(ctx, buyerID, periodStart, 10)
	require.NoError(t, err)
	require.True(t, reserved)
	require.Equal(t, 0, confirmedBefore)

	err = repo.txManager.WithTx(ctx, func(txCtx context.Context) error {
		return repo.confirmQuotaInTx(txCtx, repo.q(txCtx), reservationID, buyerID, periodStart)
	})
	require.NoError(t, err)

	// The reservation is gone and usage_count is durable, so a fresh
	// ReserveQuota at limit=1 must now see it as confirmed (not pending) usage.
	_, reserved2, confirmedBefore2, err := repo.ReserveQuota(ctx, buyerID, periodStart, 1)
	require.NoError(t, err)
	require.False(t, reserved2, "confirmed usage counts against the limit")
	require.Equal(t, 1, confirmedBefore2, "usage_count was incremented and the reservation row is gone")
}

// usageCountOf 讀出 (buyer, period) 目前 durable 的用量，0 表示還沒有那一列。
// 額度接線的所有斷言最終都要落到這個數字上——它就是「這個買家這個月被計了幾次
// 費」的唯一權威。
func usageCountOf(ctx context.Context, t *testing.T, pool *pgxpool.Pool, buyerID identity.BuyerID, periodStart time.Time) int {
	t.Helper()
	var n int
	err := pool.QueryRow(ctx,
		`SELECT usage_count FROM procurement_usage WHERE buyer_id = $1 AND period_start = $2`,
		int64(buyerID), periodStart,
	).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0
	}
	require.NoError(t, err)
	return n
}

func pendingReservationCountOf(ctx context.Context, t *testing.T, pool *pgxpool.Pool, buyerID identity.BuyerID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_usage_reservations WHERE buyer_id = $1`, int64(buyerID),
	).Scan(&n))
	return n
}

// TestAppendRevision_ConfirmsQuotaReservationInSameTransaction 是 Task 12 接線的
// 正向證明：AppendRevision 帶著 QuotaReservationID 呼叫時，預約必須在它自己的交易
// 裡被轉成 durable 用量——預約列消失、usage_count +1。這條路徑先前只有
// confirmQuotaInTx 被單獨呼叫的覆蓋（TestConfirmQuotaInTx_ConvertsReservationToUsage），
// 沒有任何測試證明 AppendRevision 真的會去呼叫它；少了這一則，把
// revision.go 那一行 confirmQuotaInTx 整段刪掉，所有既有測試依然全綠。
func TestAppendRevision_ConfirmsQuotaReservationInSameTransaction(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(900010)
	periodStart := time.Now().UTC().Truncate(24 * time.Hour)

	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	reservationID, reserved, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 10)
	require.NoError(t, err)
	require.True(t, reserved)
	require.Equal(t, 1, pendingReservationCountOf(ctx, t, pool, buyerID), "前置條件：預約要真的存在")
	require.Equal(t, 0, usageCountOf(ctx, t, pool, buyerID, periodStart))

	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{Item: ptr("梨")},
		Source: "LLM_EXTRACT", ClientRequestID: "cr-quota-append-1", RequestStatus: "CLARIFYING",
		QuotaReservationID: reservationID, QuotaPeriodStart: periodStart,
	})
	require.NoError(t, err)

	require.Equal(t, 1, usageCountOf(ctx, t, pool, buyerID, periodStart),
		"AppendRevision 必須在同一個交易裡把預約確認成 durable 用量")
	require.Zero(t, pendingReservationCountOf(ctx, t, pool, buyerID),
		"確認之後預約列必須消失，否則同一格會被 pending 與 confirmed 各算一次")
}

// TestAppendRevision_IdempotentReplay_DoesNotDoubleCountQuota 是 Task 12 最重要的
// 負向證明，對應 Task 9 reviewer 點名的風險：confirmQuotaInTx 裡的
// IncrementProcurementUsage 是無條件 +1，如果那一行被放在 AppendRevision 交易體中
// 「冪等命中直接 return」分支的前面（或被放進兩個分支），同一個 client_request_id
// 重送就會把同一輪計費兩次。這裡用同一組 (buyer_id, client_request_id) 重送，
// 並且刻意帶上第二筆真實預約——第二次呼叫必須完全不動用量，那筆預約留待 TTL 過期。
func TestAppendRevision_IdempotentReplay_DoesNotDoubleCountQuota(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(900011)
	periodStart := time.Now().UTC().Truncate(24 * time.Hour)

	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	firstReservation, reserved, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 10)
	require.NoError(t, err)
	require.True(t, reserved)

	in := AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{Item: ptr("梨")},
		Source: "LLM_EXTRACT", ClientRequestID: "cr-quota-replay-1", RequestStatus: "CLARIFYING",
		QuotaReservationID: firstReservation, QuotaPeriodStart: periodStart,
	}
	firstRev, err := repo.AppendRevision(ctx, in)
	require.NoError(t, err)
	require.Equal(t, 1, usageCountOf(ctx, t, pool, buyerID, periodStart))

	// 重送：同一個冪等鍵，但帶一筆全新的預約（模擬呼叫端在重送前又預約了一次）。
	secondReservation, reserved, _, err := repo.ReserveQuota(ctx, buyerID, periodStart, 10)
	require.NoError(t, err)
	require.True(t, reserved)
	require.NotEqual(t, firstReservation, secondReservation)

	replayIn := in
	replayIn.QuotaReservationID = secondReservation
	replayRev, err := repo.AppendRevision(ctx, replayIn)
	require.NoError(t, err)

	require.Equal(t, firstRev.ID, replayRev.ID, "前置條件：第二次必須真的走冪等重播路徑")
	require.Equal(t, 1, usageCountOf(ctx, t, pool, buyerID, periodStart),
		"冪等重播不得再計費一次——confirmQuotaInTx 必須排在交易體的冪等 return 之後")
	require.Equal(t, 1, pendingReservationCountOf(ctx, t, pool, buyerID),
		"重播沒有確認任何東西，第二筆預約應原封不動留著等 TTL 過期")
}

// TestAppendRevision_WithoutReservation_DoesNotTouchQuota 釘住
// QuotaReservationID == 0 的零值出口（見 AppendInput 註解）：沒有預約要確認時
// 絕不能無條件呼叫 confirmQuotaInTx——它會刪不到任何預約列，卻照樣把 usage_count
// +1，等於對一個從來沒預約過的 turn 計費。
func TestAppendRevision_WithoutReservation_DoesNotTouchQuota(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(900012)
	periodStart := time.Now().UTC().Truncate(24 * time.Hour)

	req, _, err := repo.CreateRequest(ctx, buyerID)
	require.NoError(t, err)

	_, err = repo.AppendRevision(ctx, AppendInput{
		RequestID: req.ID, BuyerID: buyerID, Slots: domain.Slots{Item: ptr("梨")},
		Source: "LLM_EXTRACT", ClientRequestID: "cr-quota-none-1", RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	require.Equal(t, 0, usageCountOf(ctx, t, pool, buyerID, periodStart),
		"沒有預約就不得產生任何用量")
	require.Zero(t, pendingReservationCountOf(ctx, t, pool, buyerID))
}
